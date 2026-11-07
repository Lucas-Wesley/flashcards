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

func TestLeitorDeFrasesComTraducao(t *testing.T) {
	arquivo, err := os.Open("testdata/frases.html")
	if err != nil {
		t.Fatal(err)
	}
	defer arquivo.Close()
	pagina, err := html.Parse(arquivo)
	if err != nil {
		t.Fatal(err)
	}
	base, _ := url.Parse("https://blog.exemplo.com/run-into-o-que-significa/")
	titulo, frases := lerFrasesComTraducao(pagina, base)
	if titulo != "Run Into" {
		t.Errorf("título: %q", titulo)
	}
	// A "resposta rápida", o título do sentido e o "Baixe o material" não são exemplos.
	esperadas := []fraseDeExemplo{
		{"I ran into an old friend yesterday.", "Eu encontrei um velho amigo por acaso ontem.", "https://blog.exemplo.com/audio/01-ran-into.mp3?t=1"},
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
	if c.Frente != "I ran into an old friend yesterday." || c.Tags != "run-into phrasal-verb" || !strings.HasSuffix(c.AudioURL, "/audio/01-ran-into.mp3?t=1") {
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
	if g.Proposta.NomeBaralho != "Run Into" || len(g.Proposta.Cartoes) != 3 || g.PodeRefazer {
		t.Fatalf("baralho novo: %+v (pode refazer: %v)", g.Proposta, g.PodeRefazer)
	}
	// Excluir apaga o pedido.
	excluido := "/api/geracoes/" + strconv.FormatInt(g.ID, 10)
	if s := e.pedir("DELETE", excluido, nil, nil); s != http.StatusNoContent {
		t.Errorf("excluir: %d", s)
	}
	if s := e.pedir("GET", excluido, nil, nil); s != http.StatusNotFound {
		t.Errorf("pedido excluído ainda aparece: %d", s)
	}

	if s := e.pedir("POST", "/api/links", map[string]string{"url": "ftp://exemplo.com/x"}, nil); s != http.StatusBadRequest {
		t.Errorf("não é link: %d", s)
	}
	if s := e.pedir("POST", "/api/links", map[string]string{"url": e.wiki.URL + "/foto.png"}, nil); s != http.StatusBadRequest {
		t.Errorf("link de uma imagem: %d", s)
	}
	if s := e.pedir("POST", "/api/links", map[string]string{"url": e.wiki.URL + "/vazia/"}, nil); s != http.StatusBadRequest {
		t.Errorf("página sem texto: %d", s)
	}
	if s := e.pedir("POST", "/api/notas", dadosDaNota{BaralhoID: b.ID, Frente: "x", Audio: "../x.mp3"}, nil); s != http.StatusBadRequest {
		t.Errorf("áudio inválido aceito: %d", s)
	}
}

func TestLinkComMuitasFrasesAIAEscolheAsMaisDidaticas(t *testing.T) {
	e := novoAmbiente(t)
	link := e.wiki.URL + "/shoot-up/"
	var g geracao
	if s := e.pedir("POST", "/api/links", map[string]string{"url": link}, &g); s != http.StatusCreated {
		t.Fatalf("link: %d", s)
	}
	if g.Origem != "link" || g.Estado != naFila || g.Quantidade != 3 || g.Proposta != nil || !g.PodeRefazer {
		t.Fatalf("pedido na fila: %+v", g)
	}
	id := strconv.FormatInt(g.ID, 10)
	g = e.esperarGeracao(g.ID, emRevisao)
	// A IA devolveu 5, 2, 99, 2 e 6: fica com 2, 5 e 6, na ordem da página, sem mudar as frases.
	var frentes []string
	for _, c := range g.Proposta.Cartoes {
		frentes = append(frentes, c.Frente)
	}
	if strings.Join(frentes, " | ") != "Example 2 shot up. | Example 5 shot up. | Example 6 shot up." {
		t.Fatalf("escolhidas: %v", frentes)
	}
	c := g.Proposta.Cartoes[1]
	if c.Verso != "Exemplo 5 disparou." || !strings.HasSuffix(c.AudioURL, "/audio/05.mp3") || c.Tags != "shoot-up phrasal-verb" || g.Proposta.NomeBaralho != "Shoot Up" {
		t.Errorf("cartão: %+v / %q", c, g.Proposta.NomeBaralho)
	}
	obs := g.Proposta.Observacoes
	if !strings.Contains(obs, "3 das 7 frases de exemplo de “Shoot Up”, as mais didáticas") || !strings.Contains(obs, "Uma de cada situação.") ||
		!strings.Contains(obs, "Bash,Read") || strings.Contains(obs, "Ferramentas: WebSearch") {
		t.Errorf("observações (e a IA sem internet): %q", obs)
	}

	// Refazer pede outra escolha à IA, com as mesmas frases.
	if s := e.pedir("POST", "/api/geracoes/"+id+"/refazer", nil, &g); s != http.StatusOK || g.Estado != naFila {
		t.Fatalf("refazer: %d %+v", s, g)
	}
	if g = e.esperarGeracao(g.ID, emRevisao); len(g.Proposta.Cartoes) != 3 {
		t.Errorf("nova escolha: %+v", g.Proposta)
	}
}

func TestTextoDaPagina(t *testing.T) {
	pagina, _ := html.Parse(strings.NewReader(`<html><head><title>T</title><style>p{}</style></head><body><nav>Menu</nav>
		<main><p>Use <strong>break the ice</strong> assim:</p><audio src="/a.mp3"></audio><p>Fim.</p><audio src="/a.mp3"></audio></main></body></html>`))
	base, _ := url.Parse("https://site.exemplo/post/")
	texto, audios := textoDaPagina(pagina, base)
	if texto != "Use **break the ice** assim:\n[áudio 1] Fim." || len(audios) != 1 || audios[0] != "https://site.exemplo/a.mp3" {
		t.Errorf("texto: %q, áudios: %v", texto, audios)
	}
	if semNomeDoSite("Break the Ice – Inglês Diário") != "Break the Ice" {
		t.Error("nome do site no título")
	}
}

func TestPaginaSemFrasesProntasAIATiraOsExemplos(t *testing.T) {
	e := novoAmbiente(t)
	ingles := e.criarBaralho("Inglês")
	outro := e.criarBaralho("Bandeiras")
	var g geracao
	if s := e.pedir("POST", "/api/links", map[string]string{"url": e.wiki.URL + "/artigo/", "baralho_id": outro.ID}, &g); s != http.StatusCreated {
		t.Fatalf("link: %d", s)
	}
	if g.Estado != naFila {
		t.Fatalf("pedido na fila: %+v", g)
	}
	g = e.esperarGeracao(g.ID, emRevisao)
	p := g.Proposta
	if p.NomeBaralho != "Break the Ice" || len(p.Cartoes) != 2 {
		t.Fatalf("proposta: %+v", p)
	}
	c := p.Cartoes[0]
	if c.Frente != "He told a joke to break the ice." || c.Verso != "Ele contou uma piada para quebrar o gelo." ||
		!strings.HasSuffix(c.AudioURL, "/audio/gelo.mp3") || p.Cartoes[1].AudioURL != "" || c.Tags != "break-the-ice" {
		t.Errorf("cartão: %+v / %+v", c, p.Cartoes[1])
	}
	if !strings.Contains(p.Observacoes, "tiradas pela IA de “Break the Ice”") || !strings.Contains(p.Observacoes, "Recebeu o texto da página.") ||
		strings.Contains(p.Observacoes, "Menu do site") {
		t.Errorf("observações: %q", p.Observacoes)
	}

	// Na revisão, o baralho muda para o Inglês; um áudio que não veio da página não é baixado.
	cartoes := p.Cartoes
	cartoes[1].AudioURL = e.wiki.URL + "/audio/outro.mp3"
	id := strconv.FormatInt(g.ID, 10)
	var res struct {
		BaralhoID string `json:"baralho_id"`
		SemAudio  int    `json:"sem_audio"`
	}
	if s := e.pedir("POST", "/api/geracoes/"+id+"/aprovar", map[string]any{"baralho_id": "nao-existe", "cartoes": cartoes}, nil); s != http.StatusBadRequest {
		t.Errorf("baralho inválido: %d", s)
	}
	if s := e.pedir("POST", "/api/geracoes/"+id+"/aprovar", map[string]any{"baralho_id": ingles.ID, "cartoes": cartoes}, &res); s != http.StatusOK {
		t.Fatalf("aprovar: %d", s)
	}
	if res.BaralhoID != ingles.ID || res.SemAudio != 1 {
		t.Errorf("aprovado em: %+v", res)
	}
	if es := e.estudo(ingles.ID); es.Cartao == nil || !nomeDeAudio.MatchString(es.Cartao.Audio) {
		t.Errorf("estudo no Inglês: %+v", es.Cartao)
	}

	// Pedido para o Inglês, aprovado num baralho novo escolhido na revisão.
	e.pedir("POST", "/api/links", map[string]string{"url": e.wiki.URL + "/run-into/", "baralho_id": ingles.ID}, &g)
	id = strconv.FormatInt(g.ID, 10)
	if s := e.pedir("POST", "/api/geracoes/"+id+"/aprovar", map[string]any{"baralho_id": "", "cartoes": g.Proposta.Cartoes}, nil); s != http.StatusBadRequest {
		t.Errorf("baralho novo sem nome: %d", s)
	}
	if s := e.pedir("POST", "/api/geracoes/"+id+"/aprovar", map[string]any{"baralho_id": "", "nome_baralho": "Phrasal verbs", "cartoes": g.Proposta.Cartoes}, &res); s != http.StatusOK || res.BaralhoID == ingles.ID {
		t.Errorf("baralho novo: %d %+v", s, res)
	}
}
