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
		if r.URL.Path == "/foto.png" {
			w.Write(pngDeTeste)
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
		wiki: e.wiki.URL + "/w/api.php", hostsDeImagem: []string{wikiURL.Host}}
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
