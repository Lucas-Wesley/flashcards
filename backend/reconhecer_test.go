package main

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// F39: sem quantidade, a IA decide; o cartão de reconhecer tem a imagem na frente (um arquivo exato
// do Commons, ou a busca pelo nome quando ele não existe) e a frente pode se repetir no baralho.
func TestBaralhoDeReconhecerPelaImagem(t *testing.T) {
	e := novoAmbiente(t)
	g := e.pedirGeracao(map[string]any{"pedido": "BANDEIRAS DO MUNDO: a bandeira na frente, o nome e o mapa no verso"})
	if g.Quantidade != 0 {
		t.Fatalf("sem quantidade, fica 0 (a IA decide): %d", g.Quantidade)
	}
	g = e.esperarGeracao(g.ID, emRevisao)
	p := g.Proposta
	if !strings.Contains(p.Observacoes, "Sem quantidade.") || !strings.Contains(p.Entendimento, "bandeira na frente") || len(p.Cartoes) != 2 {
		t.Fatalf("proposta: %+v", p)
	}
	japao, brasil := p.Cartoes[0], p.Cartoes[1]
	if !strings.HasSuffix(japao.ImagemFrenteURL, "/flag-japan.png") || !strings.HasSuffix(japao.ImagemURL, "/mapa-japan.png") {
		t.Fatalf("arquivos exatos do Commons: %+v", japao)
	}
	// Os arquivos exatos vêm numa consulta só (a primeira esbarrou no 429 e foi repetida).
	if n := e.consultasDeArquivos.Load(); n != 2 {
		t.Errorf("consultas de arquivos ao Commons: %d, esperava 2 (um lote, repetido depois do 429)", n)
	}
	// "Bandeira do Brasil.svg" não existe: a busca pelo nome acha a bandeira.
	if !(strings.HasSuffix(brasil.ImagemFrenteURL, "/artigo.png") || strings.HasSuffix(brasil.ImagemFrenteURL, "/atual.png")) || brasil.ImagemURL != "" {
		t.Fatalf("busca pelo nome do arquivo que não existe: %+v", brasil)
	}

	var res struct {
		BaralhoID   string `json:"baralho_id"`
		Criadas     int    `json:"criadas"`
		Atualizadas int    `json:"atualizadas"`
		SemImagem   int    `json:"sem_imagem"`
	}
	id := strconv.FormatInt(g.ID, 10)
	if s := e.pedir("POST", "/api/geracoes/"+id+"/aprovar", map[string]any{"nome_baralho": "Bandeiras", "cartoes": p.Cartoes}, &res); s != http.StatusOK {
		t.Fatalf("aprovar: %d", s)
	}
	// A mesma frente nos dois: com imagem na frente, o verso diferencia, e são duas notas.
	if res.Criadas != 2 || res.SemImagem != 0 {
		t.Fatalf("aprovado: %+v", res)
	}
	var notas []nota
	e.pedir("GET", "/api/notas?q=Japão", nil, &notas)
	if len(notas) != 1 || !nomeDeImagem.MatchString(notas[0].ImagemFrente) || !nomeDeImagem.MatchString(notas[0].Imagem) || notas[0].ImagemFrente == notas[0].Imagem {
		t.Fatalf("nota com as duas imagens: %+v", notas)
	}
	es := e.estudo(res.BaralhoID)
	if es.Cartao == nil || es.Cartao.ImagemFrente == "" {
		t.Fatalf("o estudo mostra a imagem da frente: %+v", es.Cartao)
	}

	// Refazer depois de aprovar: os dois voltam como atualização, cada um no seu cartão.
	if s := e.pedir("POST", "/api/geracoes/"+id+"/refazer", nil, nil); s != http.StatusOK {
		t.Fatalf("refazer: %d", s)
	}
	g = e.esperarGeracao(g.ID, emRevisao)
	if g.Quantidade != 0 || !g.Proposta.Cartoes[0].Atualiza || !g.Proposta.Cartoes[1].Atualiza {
		t.Fatalf("refeito: quantidade %d, %+v", g.Quantidade, g.Proposta.Cartoes)
	}
	if s := e.pedir("POST", "/api/geracoes/"+id+"/aprovar", map[string]any{"cartoes": g.Proposta.Cartoes}, &res); s != http.StatusOK || res.Atualizadas != 2 || res.Criadas != 0 {
		t.Fatalf("aprovar a atualização: %d %+v", s, res)
	}
}

func TestQuantidadeDoPedido(t *testing.T) {
	e := novoAmbiente(t)
	if s := e.pedir("POST", "/api/geracoes", map[string]any{"pedido": "países", "quantidade": maximoDeCartoes + 1}, nil); s != http.StatusBadRequest {
		t.Errorf("acima do teto: %d", s)
	}
	g := e.pedirGeracao(map[string]any{"pedido": "os países do mundo", "quantidade": 195})
	if g.Quantidade != 195 {
		t.Fatalf("quantidade: %d", g.Quantidade)
	}
	g = e.esperarGeracao(g.ID, emRevisao)
	if strings.Contains(g.Proposta.Observacoes, "Sem quantidade.") {
		t.Errorf("com número, a IA recebe o número: %q", g.Proposta.Observacoes)
	}
	// Refazer com 0: agora a IA decide.
	id := strconv.FormatInt(g.ID, 10)
	if s := e.pedir("POST", "/api/geracoes/"+id+"/refazer", map[string]any{"pedido": "os países do mundo", "quantidade": 0}, nil); s != http.StatusOK {
		t.Fatalf("refazer: %d", s)
	}
	if g = e.esperarGeracao(g.ID, emRevisao); g.Quantidade != 0 || !strings.Contains(g.Proposta.Observacoes, "Sem quantidade.") {
		t.Fatalf("refeito sem quantidade: %d %q", g.Quantidade, g.Proposta.Observacoes)
	}
}

// Falar o pedido (F39): a gravação vai ao Whisper do Mac; com o Mac fora, ao do servidor.
func TestTranscreverOPedidoFalado(t *testing.T) {
	e := novoAmbiente(t)
	if s := e.enviarFala([]byte("audio")); s != http.StatusServiceUnavailable {
		t.Errorf("sem Whisper configurado: %d", s)
	}

	var recebeu []string
	whisper := func(quem string) *httptest.Server {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/saude" {
				w.Write([]byte(`{"ok":true}`))
				return
			}
			arquivo, _, err := r.FormFile("file")
			if err != nil {
				http.Error(w, "sem arquivo", http.StatusBadRequest)
				return
			}
			dados, _ := io.ReadAll(arquivo)
			recebeu = append(recebeu, quem+":"+r.FormValue("model")+":"+r.FormValue("language")+":"+string(dados))
			w.Write([]byte(`{"text":"  Quero as bandeiras dos países.  "}`))
		}))
		t.Cleanup(srv.Close)
		return srv
	}
	mac, reserva := whisper("mac"), whisper("reserva")
	e.app.cfg.whisper, e.app.cfg.whisperReserva = mac.URL, reserva.URL

	var r struct {
		Texto string `json:"texto"`
	}
	if s := e.enviarFalaEm([]byte("OPUS"), &r); s != http.StatusOK || r.Texto != "Quero as bandeiras dos países." {
		t.Fatalf("transcrever: %d %q", s, r.Texto)
	}
	if len(recebeu) != 1 || recebeu[0] != "mac:"+modeloDoMac+":pt:OPUS" {
		t.Fatalf("o Mac devia transcrever: %v", recebeu)
	}

	mac.Close() // o Mac desligado
	if s := e.enviarFalaEm([]byte("OPUS"), &r); s != http.StatusOK || len(recebeu) != 2 || recebeu[1] != "reserva:"+modeloDaReserva+":pt:OPUS" {
		t.Fatalf("a reserva devia transcrever: %d %v", s, recebeu)
	}
}

func (e *ambiente) enviarFala(audio []byte) int { return e.enviarFalaEm(audio, nil) }

func (e *ambiente) enviarFalaEm(audio []byte, destino any) int {
	var corpo bytes.Buffer
	mw := multipart.NewWriter(&corpo)
	parte, _ := mw.CreateFormFile("audio", "fala.webm")
	parte.Write(audio)
	mw.Close()
	res, err := http.Post(e.srv.URL+"/api/transcrever", mw.FormDataContentType(), &corpo)
	if err != nil {
		e.t.Fatal(err)
	}
	defer res.Body.Close()
	if destino != nil && res.StatusCode == http.StatusOK {
		jsonDecode(res, destino)
	}
	return res.StatusCode
}
