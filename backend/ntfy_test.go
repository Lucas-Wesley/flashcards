package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// ntfyFalso guarda o que o app publica.
func ntfyFalso(t *testing.T) (endereco string, recebidos chan map[string]any) {
	t.Helper()
	recebidos = make(chan map[string]any, 10)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var m map[string]any
		json.NewDecoder(r.Body).Decode(&m)
		recebidos <- m
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/flashcards", recebidos
}

func receberAviso(t *testing.T, recebidos chan map[string]any) map[string]any {
	t.Helper()
	select {
	case m := <-recebidos:
		return m
	case <-time.After(10 * time.Second):
		t.Fatal("nenhum aviso chegou")
		return nil
	}
}

// F35: a proposta pronta para revisar e a geração que falhou avisam no celular.
func TestAvisosDaGeracao(t *testing.T) {
	e := novoAmbiente(t)
	endereco, recebidos := ntfyFalso(t)
	e.app.avisos = novoAvisador(endereco)

	g := e.pedirGeracao(map[string]any{"pedido": "presidentes", "quantidade": 5})
	e.esperarGeracao(g.ID, emRevisao)
	m := receberAviso(t, recebidos)
	if m["topic"] != "flashcards" || m["title"] != "presidentes" || m["message"] != "Cartões prontos para revisar." ||
		m["click"] != "/#/geracao/"+strconv.FormatInt(g.ID, 10) || m["priority"] != nil {
		t.Errorf("proposta pronta: %v", m)
	}

	t.Setenv("CLAUDE_FALSO_FALHA", "1")
	g = e.pedirGeracao(map[string]any{"pedido": "capitais", "quantidade": 5})
	e.esperarGeracao(g.ID, falhou)
	m = receberAviso(t, recebidos)
	if !strings.HasPrefix(m["message"].(string), "A geração falhou: ") || m["priority"] != float64(4) {
		t.Errorf("falha: %v", m)
	}
}
