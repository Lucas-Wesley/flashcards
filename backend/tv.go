package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Estudar na TV (F37). O celular continua sendo o estudo: a cada cartão, resposta mostrada ou fim,
// ele manda a cena (PUT /api/tv) e o servidor a repassa à página da TV, que só mostra. A página fica
// numa porta própria, em HTTP simples (ADDR_TV): o navegador da TV não aceita a CA do homelab. Por
// ela só passam a página, a cena e as imagens dos cartões. Uma TV só na casa: não há pareamento.

//go:embed tv.html
var paginaDaTV []byte

// A cena é o que o celular mandou; o servidor não a interpreta, só guarda a última e a repassa.
type telaTV struct {
	mu       sync.Mutex
	cena     json.RawMessage
	ouvintes map[chan json.RawMessage]struct{}
}

var cenaVazia = json.RawMessage(`{"ativo":false}`)

func novaTelaTV() *telaTV {
	return &telaTV{cena: cenaVazia, ouvintes: map[chan json.RawMessage]struct{}{}}
}

func (t *telaTV) mudar(cena json.RawMessage) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.cena = cena
	for c := range t.ouvintes {
		select {
		case c <- cena:
		default: // uma TV lenta perde a cena do meio, não a última: ela é relida a cada envio
			select {
			case <-c:
			default:
			}
			c <- cena
		}
	}
}

func (t *telaTV) ouvir() (chan json.RawMessage, json.RawMessage, func()) {
	t.mu.Lock()
	defer t.mu.Unlock()
	c := make(chan json.RawMessage, 1)
	t.ouvintes[c] = struct{}{}
	return c, t.cena, func() {
		t.mu.Lock()
		delete(t.ouvintes, c)
		t.mu.Unlock()
	}
}

func (t *telaTV) atual() (json.RawMessage, int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.cena, len(t.ouvintes)
}

// handleTV: GET /api/tv, o endereço da página e se há uma TV aberta nela.
func (a *app) handleTV(w http.ResponseWriter, r *http.Request) {
	cena, telas := a.tv.atual()
	writeJSON(w, http.StatusOK, map[string]any{"endereco": a.cfg.enderecoDaTV, "telas": telas, "cena": cena})
}

// handleMudarTV: PUT /api/tv, a cena nova; {"ativo": false} volta a TV para a espera.
func (a *app) handleMudarTV(w http.ResponseWriter, r *http.Request) {
	b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
	var cena struct {
		Ativo *bool `json:"ativo"`
	}
	if err != nil || json.Unmarshal(b, &cena) != nil || cena.Ativo == nil {
		erroJSON(w, http.StatusBadRequest, "Pedido inválido.")
		return
	}
	var linha bytes.Buffer // no Server-Sent Events, a cena vai numa linha só
	json.Compact(&linha, b)
	a.tv.mudar(json.RawMessage(linha.Bytes()))
	w.WriteHeader(http.StatusNoContent)
}

// servidorDaTV: a porta da TV, só com a página, a cena (Server-Sent Events) e as imagens.
func servidorDaTV(a *app) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Write(paginaDaTV)
	})
	mux.HandleFunc("GET /cena", a.handleCenaDaTV)
	mux.HandleFunc("GET /imagens/{nome}", a.handleImagem)
	return mux
}

// handleCenaDaTV manda a cena atual e cada mudança; um comentário a cada 25 s mantém a conexão.
func (a *app) handleCenaDaTV(w http.ResponseWriter, r *http.Request) {
	f, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "sem streaming", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	c, cena, sair := a.tv.ouvir()
	defer sair()
	enviar := func(cena json.RawMessage) {
		fmt.Fprintf(w, "data: %s\n\n", cena)
		f.Flush()
	}
	enviar(cena)
	vivo := time.NewTicker(25 * time.Second)
	defer vivo.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case cena := <-c:
			enviar(cena)
		case <-vivo.C:
			fmt.Fprint(w, ": vivo\n\n")
			f.Flush()
		}
	}
}
