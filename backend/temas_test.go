package main

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"
)

// Estudar um tema (F38) mistura só os baralhos dele; desfazer volta a última resposta do tema,
// mesmo que outro baralho tenha sido respondido depois.
func TestEstudarUmTema(t *testing.T) {
	e := novoAmbiente(t)
	phrasal := e.criarBaralho("Phrasal verbs")
	pronuncia := e.criarBaralho("Pronúncia")
	historia := e.criarBaralho("História")
	for _, b := range []baralho{phrasal, pronuncia} {
		if s := e.pedir("PATCH", "/api/baralhos/"+b.ID, map[string]string{"tema": " Inglês "}, nil); s != http.StatusOK {
			t.Fatalf("tema: %d", s)
		}
	}
	for i := range 2 {
		e.criarNota(phrasal.ID, "basico", fmt.Sprintf("pv %d", i), "x")
		e.criarNota(pronuncia.ID, "basico", fmt.Sprintf("pr %d", i), "x")
		e.criarNota(historia.ID, "basico", fmt.Sprintf("hi %d", i), "x")
	}
	var b baralho
	e.pedir("GET", "/api/baralhos/"+phrasal.ID, nil, &b)
	if b.Tema != "Inglês" {
		t.Fatalf("o tema não ficou: %q", b.Tema)
	}

	var es estudo
	e.pedir("GET", "/api/estudo?tema="+url.QueryEscape("Inglês"), nil, &es)
	if es.Tema != "Inglês" || es.Todos || es.Contagem.Novos != 4 {
		t.Fatalf("início do tema: %+v %+v", es.juntos, es.Contagem)
	}
	vistos := map[string]int{}
	for es.Cartao != nil {
		vistos[es.Baralho.Nome]++
		var prox estudo
		corpo := map[string]any{"nota": 4, "repeticoes": es.Cartao.Repeticoes, "tema": "Inglês"}
		if s := e.pedir("POST", "/api/cartoes/"+es.Cartao.ID+"/responder", corpo, &prox); s != http.StatusOK {
			t.Fatalf("responder: %d", s)
		}
		es = prox
	}
	if vistos["Phrasal verbs"] != 2 || vistos["Pronúncia"] != 2 || vistos["História"] != 0 {
		t.Fatalf("só os baralhos do tema: %v", vistos)
	}

	// Uma resposta em História depois não é a que o desfazer do tema volta.
	e.responder(e.estudo(historia.ID), 4)
	var desfeito estudo
	if s := e.pedir("POST", "/api/estudo/desfazer?tema="+url.QueryEscape("Inglês"), nil, &desfeito); s != http.StatusOK {
		t.Fatalf("desfazer: %d", s)
	}
	if desfeito.Cartao == nil || desfeito.Baralho.Tema != "Inglês" || desfeito.FeitosHoje != 3 {
		t.Fatalf("desfeito: %+v", desfeito)
	}
	if h := e.estudo(historia.ID); h.FeitosHoje != 1 {
		t.Fatalf("a resposta de História devia ficar: %+v", h)
	}
}

// O aleatório (F38) sorteia do baralho, do tema ou de todos, e não muda nada no agendamento.
func TestEstudarAleatorio(t *testing.T) {
	e := novoAmbiente(t)
	ingles := e.criarBaralho("Inglês")
	historia := e.criarBaralho("História")
	e.pedir("PATCH", "/api/baralhos/"+ingles.ID, map[string]string{"tema": "Idiomas"}, nil)
	e.criarNota(ingles.ID, "invertido", "house", "casa")
	e.criarNota(historia.ID, "basico", "1500", "Cabral")
	suspensa := e.criarNota(historia.ID, "basico", "1822", "Independência")
	e.pedir("PATCH", "/api/cartoes/"+suspensa.Cartoes[0].ID, map[string]bool{"suspenso": true}, nil)
	antes := e.estudo(ingles.ID)

	sortear := func(q string) []cartaoSorteado {
		t.Helper()
		var lista []cartaoSorteado
		if s := e.pedir("GET", "/api/aleatorio"+q, nil, &lista); s != http.StatusOK {
			t.Fatalf("aleatório%s: %d", q, s)
		}
		return lista
	}
	if l := sortear(""); len(l) != 3 {
		t.Fatalf("todos, sem o suspenso: %+v", l)
	}
	l := sortear("?tema=Idiomas")
	if len(l) != 2 || l[0].Baralho != "Inglês" {
		t.Fatalf("tema: %+v", l)
	}
	for _, c := range l {
		if c.Sentido == 1 && (c.Frente != "casa" || c.Verso != "house") {
			t.Fatalf("o invertido sai invertido: %+v", c)
		}
	}
	if l := sortear("?baralho=" + historia.ID); len(l) != 1 || l[0].Frente != "1500" {
		t.Fatalf("baralho: %+v", l)
	}

	depois := e.estudo(ingles.ID)
	if depois.Contagem != antes.Contagem || depois.FeitosHoje != 0 || depois.Cartao.Repeticoes != 0 {
		t.Fatalf("o aleatório mexeu no agendamento: %+v → %+v", antes, depois)
	}
}

// Um cartão de leitura (F38) é um cartão só, com o verso obrigatório, e o estudo diz o tipo.
func TestNotaDeLeitura(t *testing.T) {
	e := novoAmbiente(t)
	b := e.criarBaralho("Curiosidades")
	if s := e.pedir("POST", "/api/notas", map[string]any{"baralho_id": b.ID, "tipo": "leitura", "frente": "Qual a origem do nome Ray-Ban?"}, nil); s != http.StatusBadRequest {
		t.Fatalf("leitura sem verso: %d", s)
	}
	n := e.criarNota(b.ID, "leitura", "Qual a origem do nome Ray-Ban?", "Em 1937, a Bausch & Lomb…\n\nO nome vem de banir os raios.")
	if len(n.Cartoes) != 1 {
		t.Fatalf("leitura dá um cartão: %+v", n.Cartoes)
	}
	if es := e.estudo(b.ID); es.Cartao.Tipo != "leitura" {
		t.Fatalf("o estudo devia dizer o tipo: %+v", es.Cartao)
	}
	// Pedir à IA ou de um link: só perguntas.
	if s := e.pedir("POST", "/api/geracoes", map[string]any{"pedido": "Ray-Ban", "quantidade": 3, "tipo": "leitura"}, nil); s != http.StatusBadRequest {
		t.Fatalf("geração de leitura pelo pedido: %d", s)
	}
}
