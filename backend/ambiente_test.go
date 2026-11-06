package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

// ambiente: servidor de teste com banco próprio e um relógio que o teste avança à mão.
type ambiente struct {
	t     *testing.T
	srv   *httptest.Server
	wiki  *httptest.Server
	app   *app
	dados string
	agora time.Time
	fuso  *time.Location
}

// Um PNG de 1×1 pixel.
var pngDeTeste, _ = base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==")

func novoAmbiente(t *testing.T) *ambiente {
	t.Helper()
	db, err := abrirBanco(filepath.Join(t.TempDir(), "teste.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	fuso, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Fatal(err)
	}
	e := &ambiente{t: t, fuso: fuso, agora: time.Date(2026, 11, 6, 10, 0, 0, 0, fuso)}

	// Wikipédia falsa: a busca acha o artigo do Lula com foto; a foto é um PNG de verdade.
	e.wiki = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// O mesmo servidor faz o papel dos sites dos links.
		switch {
		case r.URL.Path == "/foto.png":
			w.Write(pngDeTeste)
			return
		case r.URL.Path == "/shoot-up/":
			// Sete exemplos: a IA escolhe os 3 mais didáticos.
			fmt.Fprint(w, "<html><head><title>Shoot Up | O que significa este phrasal verb? - Blog de Inglês</title></head><body><h1>Shoot Up</h1>")
			for i := 1; i <= 7; i++ {
				fmt.Fprintf(w, `<p><em><strong>Example %d shot up.</strong><br>Exemplo %d disparou.</em></p><figure class="wp-block-audio"><audio src="/audio/%02d.mp3"></audio></figure>`, i, i, i)
			}
			fmt.Fprint(w, "</body></html>")
		case r.URL.Path == "/run-into/":
			pagina, _ := os.ReadFile("testdata/frases.html")
			w.Write(bytes.ReplaceAll(pagina, []byte("https://SITE"), []byte(e.wiki.URL)))
			return
		case r.URL.Path == "/artigo/":
			// Um site que não mostra as frases do jeito comum: a IA tira os exemplos do texto.
			fmt.Fprint(w, `<html><head><title>Break the Ice – Inglês Diário</title><script>var x = "não é texto";</script></head><body>
				<nav>Menu do site</nav><article><h1>Break the Ice</h1><p>Usamos <b>break the ice</b> para quebrar o gelo.</p>
				<ul><li>Exemplo: <i>He told a joke to break the ice.</i> (Ele contou uma piada para quebrar o gelo.)</li></ul>
				<audio><source src="/audio/gelo.mp3"></audio><footer>Rodapé</footer></article></body></html>`)
			return
		case r.URL.Path == "/vazia/":
			fmt.Fprint(w, "<html><head><title>Nada</title></head><body><nav>Menu</nav></body></html>")
			return
		case strings.HasPrefix(r.URL.Path, "/audio/"):
			w.Write(append([]byte("ID3\x04\x00\x00\x00\x00\x00\x00"), r.URL.Path...))
			return
		}
		// Commons falso: a bandeira atual e a do Império; com o ano, só a do Império.
		if r.URL.Path == "/commons/api.php" {
			busca := r.URL.Query().Get("gsrsearch")
			switch {
			case strings.Contains(busca, "Bandeira do Brasil 1822"):
				fmt.Fprintf(w, `{"query":{"pages":[{"title":"File:Flag of Brazil (1822–1853).svg","index":1,"imageinfo":[{"thumburl":"%s/imperio.png","mime":"image/svg+xml","extmetadata":{"LicenseShortName":{"value":"Public domain"}}}]}]}}`, e.wiki.URL)
			case strings.Contains(busca, "Bandeira do Brasil"):
				fmt.Fprintf(w, `{"query":{"pages":[{"title":"File:Flag of Brazil.svg","index":1,"imageinfo":[{"thumburl":"%s/atual.png","mime":"image/svg+xml","extmetadata":{"Artist":{"value":"<a href=\"x\">Fulano</a>"}}}]},{"title":"File:Hino.pdf","index":2,"imageinfo":[{"thumburl":"%s/hino.png","mime":"application/pdf"}]}]}}`, e.wiki.URL, e.wiki.URL)
			default:
				w.Write([]byte(`{"batchcomplete":true}`))
			}
			return
		}
		if r.URL.Path == "/openverse/" {
			if strings.Contains(r.URL.Query().Get("q"), "Bandeira do Brasil") && r.URL.Query().Get("excluded_source") == "wikimedia" {
				fmt.Fprintf(w, `{"results":[{"title":"Bandeira na praia","thumbnail":"%s/praia.png","license":"by-nc","license_version":"2.0","creator":"alguém","source":"flickr"}]}`, e.wiki.URL)
				return
			}
			w.Write([]byte(`{"results":[]}`))
			return
		}
		if strings.Contains(r.URL.Query().Get("gsrsearch"), "Bandeira do Brasil") {
			fmt.Fprintf(w, `{"query":{"pages":[{"title":"Bandeira do Brasil","index":1,"thumbnail":{"source":"%s/artigo.png"}},{"title":"Bandeira do Império do Brasil","index":2,"thumbnail":{"source":"%s/artigo-imperio.png"}}]}}`, e.wiki.URL, e.wiki.URL)
			return
		}
		if strings.Contains(r.URL.Query().Get("gsrsearch"), "Lula") {
			fmt.Fprintf(w, `{"query":{"pages":[{"title":"Lula (molusco)","index":2},{"title":"Luiz Inácio Lula da Silva","index":1,"thumbnail":{"source":"%s/foto.png"}}]}}`, e.wiki.URL)
			return
		}
		w.Write([]byte(`{"batchcomplete":true}`))
	}))
	t.Cleanup(e.wiki.Close)
	wikiURL, _ := url.Parse(e.wiki.URL)

	dados := t.TempDir()
	e.dados = dados
	cfg := config{dados: dados, regras: absoluto("../regras"), claude: absoluto("testdata/claude-falso"),
		wiki: e.wiki.URL + "/w/api.php", commons: e.wiki.URL + "/commons/api.php", openverse: e.wiki.URL + "/openverse/", hostsDeImagem: []string{wikiURL.Host}}
	a, err := novoApp(db, novoRelogio(fuso, func() time.Time { return e.agora }), cfg)
	if err != nil {
		t.Fatal(err)
	}
	e.app = a
	ctx, parar := context.WithCancel(context.Background())
	go a.fila.rodar(ctx)
	e.srv = httptest.NewServer(newServer(a, fstest.MapFS{"index.html": {Data: []byte("<div id=\"app\"></div>")}}, "teste"))
	t.Cleanup(func() {
		e.srv.Close()
		parar()
	})
	return e
}

func (e *ambiente) passar(d time.Duration) { e.agora = e.agora.Add(d) }

// pedir faz a chamada e decodifica a resposta em "destino" (se não for nil). Devolve o status.
func (e *ambiente) pedir(metodo, caminho string, corpo, destino any) int {
	e.t.Helper()
	var leitor *bytes.Reader
	if corpo != nil {
		b, _ := json.Marshal(corpo)
		leitor = bytes.NewReader(b)
	} else {
		leitor = bytes.NewReader(nil)
	}
	req, _ := http.NewRequest(metodo, e.srv.URL+caminho, leitor)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer res.Body.Close()
	if destino != nil && res.StatusCode < 300 {
		if err := json.NewDecoder(res.Body).Decode(destino); err != nil {
			e.t.Fatalf("%s %s: %v", metodo, caminho, err)
		}
	}
	return res.StatusCode
}

func (e *ambiente) criarBaralho(nome string) baralho {
	e.t.Helper()
	var b baralho
	if s := e.pedir("POST", "/api/baralhos", map[string]string{"nome": nome}, &b); s != http.StatusCreated {
		e.t.Fatalf("criar baralho: %d", s)
	}
	return b
}

func (e *ambiente) criarNota(baralhoID, tipo, frente, verso string) nota {
	e.t.Helper()
	var n nota
	if s := e.pedir("POST", "/api/notas", dadosDaNota{BaralhoID: baralhoID, Tipo: tipo, Frente: frente, Verso: verso}, &n); s != http.StatusCreated {
		e.t.Fatalf("criar nota: %d", s)
	}
	return n
}

func (e *ambiente) estudo(baralhoID string) estudo {
	e.t.Helper()
	var es estudo
	if s := e.pedir("GET", "/api/baralhos/"+baralhoID+"/estudo", nil, &es); s != http.StatusOK {
		e.t.Fatalf("estudo: %d", s)
	}
	return es
}

// responder responde o cartão da vez e devolve a tela seguinte.
func (e *ambiente) responder(es estudo, nota int) estudo {
	e.t.Helper()
	if es.Cartao == nil {
		e.t.Fatal("sem cartão para responder")
	}
	var prox estudo
	corpo := map[string]any{"nota": nota, "repeticoes": es.Cartao.Repeticoes}
	if s := e.pedir("POST", "/api/cartoes/"+es.Cartao.ID+"/responder", corpo, &prox); s != http.StatusOK {
		e.t.Fatalf("responder: %d", s)
	}
	return prox
}

func (e *ambiente) nota(id string) nota {
	e.t.Helper()
	var n nota
	if s := e.pedir("GET", "/api/notas/"+id, nil, &n); s != http.StatusOK {
		e.t.Fatalf("nota: %d", s)
	}
	return n
}

func jsonDecode(res *http.Response, destino any) {
	json.NewDecoder(res.Body).Decode(destino)
}
