package main

import (
	"net/http"
	"testing"
	"time"
)

func TestLimiteDeNovosPorDia(t *testing.T) {
	e := novoAmbiente(t)
	b := e.criarBaralho("Inglês")
	e.pedir("PATCH", "/api/baralhos/"+b.ID, map[string]int{"novos_por_dia": 2}, nil)
	for _, p := range []string{"casa", "cão", "gato"} {
		e.criarNota(b.ID, "basico", p, "x")
	}

	es := e.estudo(b.ID)
	if es.Contagem.Novos != 2 || es.Cartao.Frente != "casa" {
		t.Fatalf("início: %+v, cartão %q", es.Contagem, es.Cartao.Frente)
	}
	es = e.responder(es, 4)
	if es.Cartao.Frente != "cão" {
		t.Fatalf("segundo: %q", es.Cartao.Frente)
	}
	es = e.responder(es, 4)
	if es.Cartao != nil || es.Contagem.Novos != 0 || es.FeitosHoje != 2 {
		t.Fatalf("limite de novos não segurou: %+v %+v", es.Cartao, es.Contagem)
	}

	// Às 3h ainda é o mesmo dia de estudo; às 4h o limite volta.
	e.agora = time.Date(2026, 11, 7, 3, 59, 0, 0, e.fuso)
	if es := e.estudo(b.ID); es.Contagem.Novos != 0 {
		t.Fatalf("às 3h59: %+v", es.Contagem)
	}
	e.agora = time.Date(2026, 11, 7, 4, 0, 0, 0, e.fuso)
	if es := e.estudo(b.ID); es.Contagem.Novos != 1 || es.Cartao.Frente != "gato" {
		t.Fatalf("às 4h: %+v", es.Contagem)
	}
}

func TestAprendendoVemAntesDosNovos(t *testing.T) {
	e := novoAmbiente(t)
	b := e.criarBaralho("Inglês")
	for _, p := range []string{"um", "dois", "três"} {
		e.criarNota(b.ID, "basico", p, "x")
	}

	es := e.responder(e.estudo(b.ID), 3) // "um" vai para 10 min
	if es.Cartao.Frente != "dois" || es.Contagem.Aprendendo != 1 || es.Contagem.Novos != 2 {
		t.Fatalf("depois de um: %q %+v", es.Cartao.Frente, es.Contagem)
	}
	e.passar(10 * time.Minute)
	if es := e.estudo(b.ID); es.Cartao.Frente != "um" {
		t.Fatalf("o aprendendo vencido devia vir primeiro, veio %q", es.Cartao.Frente)
	}
}

// Sem mais nada, o cartão aprendendo que vence nos próximos 20 min já aparece; depois disso, a
// tela diz quando ele vence.
func TestAprenderAdiante(t *testing.T) {
	e := novoAmbiente(t)
	b := e.criarBaralho("Inglês")
	e.criarNota(b.ID, "basico", "um", "x")

	es := e.responder(e.estudo(b.ID), 3) // 10 min
	if es.Cartao == nil || es.Cartao.Frente != "um" {
		t.Fatalf("devia mostrar o mesmo cartão adiante: %+v", es.Cartao)
	}
	es = e.responder(es, 1) // errei: 1 min
	es = e.responder(es, 3) // 10 min
	es = e.responder(es, 3) // gradua: 2 dias
	if es.Cartao != nil || es.EsperaAte != "" {
		t.Fatalf("graduado, não devia haver nada hoje: %+v %q", es.Cartao, es.EsperaAte)
	}
}

func TestRevisaoVenceNoDiaDeEstudo(t *testing.T) {
	e := novoAmbiente(t) // 6/11, 10h
	b := e.criarBaralho("Inglês")
	e.criarNota(b.ID, "basico", "um", "x")
	es := e.responder(e.estudo(b.ID), 3)
	e.passar(10 * time.Minute)
	es = e.responder(e.estudo(b.ID), 3) // vence em 8/11, 10h10
	if es.Cartao != nil {
		t.Fatal("não devia sobrar cartão")
	}

	// Em 8/11 às 5h, antes da hora exata, ele já é do dia: aparece.
	e.agora = time.Date(2026, 11, 8, 5, 0, 0, 0, e.fuso)
	es = e.estudo(b.ID)
	if es.Contagem.Revisar != 1 || es.Cartao == nil || es.Cartao.Estado != revisao {
		t.Fatalf("revisão do dia: %+v %+v", es.Contagem, es.Cartao)
	}
	// Em 7/11, não.
	e.agora = time.Date(2026, 11, 7, 23, 0, 0, 0, e.fuso)
	if es := e.estudo(b.ID); es.Cartao != nil {
		t.Fatalf("ainda não venceu: %+v", es.Cartao)
	}
}

func TestIrmaosFicamParaODiaSeguinte(t *testing.T) {
	e := novoAmbiente(t)
	b := e.criarBaralho("Inglês")
	n := e.criarNota(b.ID, "invertido", "casa", "house")
	if len(n.Cartoes) != 2 {
		t.Fatalf("nota invertida devia ter 2 cartões: %+v", n.Cartoes)
	}

	es := e.estudo(b.ID)
	if es.Contagem.Novos != 2 || es.Cartao.Frente != "casa" || es.Cartao.Verso != "house" {
		t.Fatalf("primeiro sentido: %+v %+v", es.Contagem, es.Cartao)
	}
	es = e.responder(es, 4)
	if es.Cartao != nil || es.Contagem.Novos != 0 {
		t.Fatalf("o outro sentido devia ficar para amanhã: %+v", es.Cartao)
	}
	e.passar(24 * time.Hour)
	es = e.estudo(b.ID)
	if es.Cartao == nil || es.Cartao.Frente != "house" || es.Cartao.Verso != "casa" || es.Cartao.Sentido != 1 {
		t.Fatalf("segundo sentido: %+v", es.Cartao)
	}
}

func TestDesfazerVoltaOCartao(t *testing.T) {
	e := novoAmbiente(t)
	b := e.criarBaralho("Inglês")
	e.criarNota(b.ID, "basico", "um", "x")
	e.criarNota(b.ID, "basico", "dois", "x")

	primeiro := e.estudo(b.ID)
	depois := e.responder(primeiro, 4)
	if depois.Cartao.Frente != "dois" {
		t.Fatalf("depois: %q", depois.Cartao.Frente)
	}
	// Toque duplo: a mesma resposta de novo é recusada.
	if s := e.pedir("POST", "/api/cartoes/"+primeiro.Cartao.ID+"/responder", map[string]any{"nota": 4, "repeticoes": 0}, nil); s != http.StatusConflict {
		t.Fatalf("resposta repetida: %d", s)
	}

	var volta estudo
	if s := e.pedir("POST", "/api/baralhos/"+b.ID+"/desfazer", nil, &volta); s != http.StatusOK {
		t.Fatalf("desfazer: %d", s)
	}
	if volta.Cartao.ID != primeiro.Cartao.ID || volta.Contagem.Novos != 2 || volta.FeitosHoje != 0 {
		t.Fatalf("desfeito: %+v %+v", volta.Cartao, volta.Contagem)
	}
	n := e.nota(primeiro.Cartao.NotaID)
	if c := n.Cartoes[0]; c.Estado != novo || c.DiasAgendados != 0 {
		t.Fatalf("cartão devia voltar a novo: %+v", c)
	}
	if s := e.pedir("POST", "/api/baralhos/"+b.ID+"/desfazer", nil, nil); s != http.StatusNotFound {
		t.Fatalf("sem nada para desfazer: %d", s)
	}
}

func TestSuspensoSaiDaFila(t *testing.T) {
	e := novoAmbiente(t)
	b := e.criarBaralho("Inglês")
	n := e.criarNota(b.ID, "basico", "um", "x")
	e.criarNota(b.ID, "basico", "dois", "x")

	if s := e.pedir("PATCH", "/api/cartoes/"+n.Cartoes[0].ID, map[string]bool{"suspenso": true}, nil); s != http.StatusNoContent {
		t.Fatalf("suspender: %d", s)
	}
	es := e.estudo(b.ID)
	if es.Contagem.Novos != 1 || es.Cartao.Frente != "dois" {
		t.Fatalf("suspenso na fila: %+v %q", es.Contagem, es.Cartao.Frente)
	}
	var r resumo
	e.pedir("GET", "/api/baralhos/"+b.ID, nil, &r)
	if r.Cartoes != 2 || r.Suspensos != 1 {
		t.Fatalf("resumo: %+v", r)
	}
}

func TestOpcoesDoBaralho(t *testing.T) {
	e := novoAmbiente(t)
	b := e.criarBaralho("  Inglês ")
	if b.Nome != "Inglês" || b.NovosPorDia != 20 || b.RevisoesPorDia != 200 || b.Retencao != 0.9 {
		t.Fatalf("padrão: %+v", b)
	}
	if s := e.pedir("PATCH", "/api/baralhos/"+b.ID, map[string]float64{"retencao": 0.5}, nil); s != http.StatusBadRequest {
		t.Errorf("retenção de 50%% aceita: %d", s)
	}
	if s := e.pedir("POST", "/api/baralhos", map[string]string{"nome": " "}, nil); s != http.StatusBadRequest {
		t.Errorf("baralho sem nome aceito: %d", s)
	}
	var r resumo
	e.pedir("PATCH", "/api/baralhos/"+b.ID, map[string]any{"nome": "Inglês 2", "retencao": 0.85}, &r)
	if r.Nome != "Inglês 2" || r.Retencao != 0.85 || r.NovosPorDia != 20 {
		t.Fatalf("mudado: %+v", r)
	}

	e.criarNota(b.ID, "basico", "um", "x")
	e.responder(e.estudo(b.ID), 3)
	if s := e.pedir("DELETE", "/api/baralhos/"+b.ID, nil, nil); s != http.StatusNoContent {
		t.Fatalf("apagar: %d", s)
	}
	var notas []nota
	e.pedir("GET", "/api/notas", nil, &notas)
	if len(notas) != 0 {
		t.Fatalf("notas do baralho apagado ficaram: %d", len(notas))
	}
}
