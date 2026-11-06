package main

import (
	"net/http"
	"testing"
)

func TestNotaMudaDeTipoEDeBaralho(t *testing.T) {
	e := novoAmbiente(t)
	ingles := e.criarBaralho("Inglês")
	geral := e.criarBaralho("Geral")
	n := e.criarNota(ingles.ID, "basico", " casa ", "house")
	if n.Frente != "casa" || len(n.Cartoes) != 1 {
		t.Fatalf("criada: %+v", n)
	}
	primeiro := n.Cartoes[0].ID

	var m nota
	e.pedir("PUT", "/api/notas/"+n.ID, dadosDaNota{BaralhoID: geral.ID, Tipo: "invertido", Frente: "casa", Verso: "house", Tags: "idioma, idioma substantivo"}, &m)
	if len(m.Cartoes) != 2 || m.Cartoes[0].ID != primeiro || m.BaralhoID != geral.ID || m.Tags != "idioma substantivo" {
		t.Fatalf("invertida: %+v", m)
	}
	if es := e.estudo(geral.ID); es.Contagem.Novos != 2 {
		t.Fatalf("cartões não foram para o baralho novo: %+v", es.Contagem)
	}

	e.pedir("PUT", "/api/notas/"+n.ID, dadosDaNota{BaralhoID: geral.ID, Tipo: "basico", Frente: "casa", Verso: "house"}, &m)
	if len(m.Cartoes) != 1 || m.Cartoes[0].ID != primeiro {
		t.Fatalf("de volta a básica: %+v", m.Cartoes)
	}

	if s := e.pedir("POST", "/api/notas", dadosDaNota{BaralhoID: geral.ID, Tipo: "invertido", Frente: "só frente"}, nil); s != http.StatusBadRequest {
		t.Errorf("invertida sem verso aceita: %d", s)
	}
	if s := e.pedir("POST", "/api/notas", dadosDaNota{BaralhoID: "nao-existe", Frente: "x"}, nil); s != http.StatusBadRequest {
		t.Errorf("baralho inexistente aceito: %d", s)
	}
}

func TestBuscaDeNotas(t *testing.T) {
	e := novoAmbiente(t)
	b := e.criarBaralho("Inglês")
	e.criarNota(b.ID, "basico", "casa", "house")
	e.criarNota(b.ID, "basico", "50% off", "metade")
	outro := e.criarBaralho("Outro")
	e.criarNota(outro.ID, "basico", "casarão", "mansion")

	var lista []nota
	e.pedir("GET", "/api/notas?q=casa", nil, &lista)
	if len(lista) != 2 {
		t.Fatalf("busca casa: %d", len(lista))
	}
	e.pedir("GET", "/api/notas?q=casa&baralho="+b.ID, nil, &lista)
	if len(lista) != 1 || lista[0].Verso != "house" {
		t.Fatalf("busca no baralho: %+v", lista)
	}
	e.pedir("GET", "/api/notas?q=50%25", nil, &lista)
	if len(lista) != 1 {
		t.Fatalf("o %% devia ser literal: %d", len(lista))
	}
}

func TestImportar(t *testing.T) {
	e := novoAmbiente(t)
	b := e.criarBaralho("Inglês")
	e.criarNota(b.ID, "basico", "casa", "house")

	texto := "#separator:Semicolon\n" +
		"cachorro;dog;animal\n" +
		"casa;house\n" + // repetida
		"\"gato; felino\";cat\n" +
		"sem verso\n" +
		"\n"
	var r struct{ Importadas, Repetidas, Ignoradas int }
	if s := e.pedir("POST", "/api/baralhos/"+b.ID+"/importar", map[string]string{"texto": texto, "tipo": "invertido"}, &r); s != http.StatusOK {
		t.Fatalf("importar: %d", s)
	}
	if r.Importadas != 2 || r.Repetidas != 1 || r.Ignoradas != 1 {
		t.Fatalf("resultado: %+v", r)
	}
	var lista []nota
	e.pedir("GET", "/api/notas?q=felino", nil, &lista)
	if len(lista) != 1 || lista[0].Frente != "gato; felino" || len(lista[0].Cartoes) != 2 {
		t.Fatalf("aspas: %+v", lista)
	}

	e.pedir("POST", "/api/baralhos/"+b.ID+"/importar", map[string]string{"texto": "pássaro\tbird\n"}, &r)
	if r.Importadas != 1 {
		t.Fatalf("tabulação: %+v", r)
	}
	if s := e.pedir("POST", "/api/baralhos/"+b.ID+"/importar", map[string]string{"texto": "  \n"}, nil); s != http.StatusBadRequest {
		t.Errorf("texto vazio aceito: %d", s)
	}
}

func TestSugestoesDeMelhoria(t *testing.T) {
	e := novoAmbiente(t)
	const id = "3f6c1d2e-aaaa-4bbb-8ccc-000000000001"

	var s melhoria
	if c := e.pedir("PUT", "/api/sugestoes/"+id, map[string]string{"texto": " Botão maior no estudo ", "tela": "#/estudar/x"}, &s); c != http.StatusOK {
		t.Fatalf("PUT: %d", c)
	}
	if s.Texto != "Botão maior no estudo" || s.Estado != "nova" {
		t.Fatalf("criada: %+v", s)
	}
	if c := e.pedir("PATCH", "/api/sugestoes/"+id, map[string]string{"estado": "feita", "resposta": "Aumentei."}, &s); c != http.StatusOK || s.Estado != "feita" {
		t.Fatalf("PATCH: %d %+v", c, s)
	}
	var lista []melhoria
	e.pedir("GET", "/api/sugestoes", nil, &lista)
	if len(lista) != 1 || lista[0].Resposta != "Aumentei." {
		t.Fatalf("lista: %+v", lista)
	}
	if c := e.pedir("DELETE", "/api/sugestoes/"+id, nil, nil); c != http.StatusNoContent {
		t.Fatalf("DELETE: %d", c)
	}
}
