package main

import (
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestLeitorDoMairoVergara(t *testing.T) {
	arquivo, err := os.Open("testdata/mairo.html")
	if err != nil {
		t.Fatal(err)
	}
	defer arquivo.Close()
	pagina, err := html.Parse(arquivo)
	if err != nil {
		t.Fatal(err)
	}
	base, _ := url.Parse("https://www.mairovergara.com/run-into-o-que-significa/")
	titulo, frases := lerMairoVergara(pagina, base)
	if titulo != "Run Into" {
		t.Errorf("título: %q", titulo)
	}
	// A "resposta rápida", o título do sentido e o "Baixe o material" não são exemplos.
	esperadas := []fraseDeExemplo{
		{"I ran into an old friend yesterday.", "Eu encontrei um velho amigo por acaso ontem.", "https://www.mairovergara.com/audio/01-ran-into.mp3?t=1"},
		{"Guess who I ran into at the store.", "Adivinha quem eu encontrei no mercado.\n(Também se diz bump into.)", ""},
		{"The project ran into problems.", "O projeto esbarrou em problemas.", "https://SITE/audio/03-problems.mp3"},
	}
	if len(frases) != len(esperadas) {
		t.Fatalf("frases: %+v", frases)
	}
	for i, f := range frases {
		if f != esperadas[i] {
			t.Errorf("frase %d:\n tem    %+v\n espera %+v", i, f, esperadas[i])
		}
	}
}

func TestCartoesDeUmLink(t *testing.T) {
	e := novoAmbiente(t)
	b := e.criarBaralho("Inglês")
	link := e.wiki.URL + "/run-into/"

	var g geracao
	if s := e.pedir("POST", "/api/links", map[string]string{"url": link, "baralho_id": b.ID}, &g); s != http.StatusCreated {
		t.Fatalf("link: %d", s)
	}
	if g.Origem != "link" || g.Estado != emRevisao || len(g.Proposta.Cartoes) != 3 {
		t.Fatalf("proposta do link: %+v", g)
	}
	c := g.Proposta.Cartoes[0]
	if c.Frente != "I ran into an old friend yesterday." || c.Tags != "run-into" || !strings.HasSuffix(c.AudioURL, "/audio/01-ran-into.mp3?t=1") {
		t.Fatalf("cartão: %+v", c)
	}
	if !strings.Contains(g.Proposta.Observacoes, "3 frases de exemplo de “Run Into”, 2 com áudio") {
		t.Errorf("observações: %q", g.Proposta.Observacoes)
	}
	id := strconv.FormatInt(g.ID, 10)
	if s := e.pedir("POST", "/api/geracoes/"+id+"/refazer", nil, nil); s != http.StatusConflict {
		t.Errorf("refazer um link pela IA: %d", s)
	}

	var res struct {
		SemAudio int `json:"sem_audio"`
	}
	if s := e.pedir("POST", "/api/geracoes/"+id+"/aprovar", map[string]any{"cartoes": g.Proposta.Cartoes}, &res); s != http.StatusOK || res.SemAudio != 0 {
		t.Fatalf("aprovar: %d %+v", s, res)
	}
	es := e.estudo(b.ID)
	if es.Cartao.Frente != "I ran into an old friend yesterday." || !nomeDeAudio.MatchString(es.Cartao.Audio) {
		t.Fatalf("estudo com áudio: %+v", es.Cartao)
	}
	audio, err := http.Get(e.srv.URL + "/api/audios/" + es.Cartao.Audio)
	if err != nil || audio.StatusCode != http.StatusOK || audio.Header.Get("Content-Type") != "audio/mpeg" {
		t.Fatalf("áudio: %v %+v", err, audio)
	}
	audio.Body.Close()

	// O mesmo link de novo, no mesmo baralho: nada novo.
	if s := e.pedir("POST", "/api/links", map[string]string{"url": link, "baralho_id": b.ID}, nil); s != http.StatusConflict {
		t.Errorf("link repetido: %d", s)
	}
	// Para um baralho novo, tudo de novo, com o título como nome sugerido.
	e.pedir("POST", "/api/links", map[string]string{"url": link}, &g)
	if g.Proposta.NomeBaralho != "Run Into" || len(g.Proposta.Cartoes) != 3 {
		t.Fatalf("baralho novo: %+v", g.Proposta)
	}

	if s := e.pedir("POST", "/api/links", map[string]string{"url": "https://exemplo.com/x"}, nil); s != http.StatusBadRequest {
		t.Errorf("site desconhecido: %d", s)
	}
	if s := e.pedir("POST", "/api/links", map[string]string{"url": e.wiki.URL + "/foto.png"}, nil); s != http.StatusBadRequest {
		t.Errorf("página sem frases: %d", s)
	}
	if s := e.pedir("POST", "/api/notas", dadosDaNota{BaralhoID: b.ID, Frente: "x", Audio: "../x.mp3"}, nil); s != http.StatusBadRequest {
		t.Errorf("áudio inválido aceito: %d", s)
	}
}
