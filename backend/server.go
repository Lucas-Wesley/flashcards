package main

import (
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"path"
	"strings"
)

func newServer(a *app, web fs.FS, version string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", a.handleHealth(version))

	mux.HandleFunc("GET /api/baralhos", a.handleBaralhos)
	mux.HandleFunc("POST /api/baralhos", a.handleCriarBaralho)
	mux.HandleFunc("GET /api/baralhos/{id}", a.handleBaralho)
	mux.HandleFunc("PATCH /api/baralhos/{id}", a.handleMudarBaralho)
	mux.HandleFunc("DELETE /api/baralhos/{id}", a.handleApagarBaralho)
	mux.HandleFunc("GET /api/baralhos/{id}/estudo", a.handleEstudo)
	mux.HandleFunc("POST /api/baralhos/{id}/desfazer", a.handleDesfazer)
	mux.HandleFunc("POST /api/baralhos/{id}/importar", a.handleImportar)

	mux.HandleFunc("GET /api/notas", a.handleNotas)
	mux.HandleFunc("POST /api/notas", a.handleCriarNota)
	mux.HandleFunc("GET /api/notas/{id}", a.handleNota)
	mux.HandleFunc("PUT /api/notas/{id}", a.handleMudarNota)
	mux.HandleFunc("DELETE /api/notas/{id}", a.handleApagarNota)

	mux.HandleFunc("GET /api/estudo", a.handleEstudoGeral)
	mux.HandleFunc("POST /api/estudo/desfazer", a.handleDesfazerGeral)
	mux.HandleFunc("POST /api/cartoes/{id}/responder", a.handleResponder)
	mux.HandleFunc("PATCH /api/cartoes/{id}", a.handleMudarCartao)

	mux.HandleFunc("GET /api/geracoes", a.handleGeracoes)
	mux.HandleFunc("POST /api/geracoes", a.handlePedirGeracao)
	mux.HandleFunc("GET /api/geracoes/{id}", a.handleGeracao)
	mux.HandleFunc("POST /api/geracoes/{id}/aprovar", a.handleAprovar)
	mux.HandleFunc("POST /api/geracoes/{id}/{acao}", a.handleAcaoGeracao)
	mux.HandleFunc("POST /api/textos", a.handlePedirDoTexto)

	mux.HandleFunc("POST /api/links", a.handleLink)

	mux.HandleFunc("GET /api/imagens", a.handleBuscarImagens)
	mux.HandleFunc("POST /api/imagens/baixar", a.handleBaixarImagem)
	mux.HandleFunc("POST /api/imagens", a.handleEnviarImagem)
	mux.HandleFunc("GET /api/imagens/{nome}", a.handleImagem)
	mux.HandleFunc("GET /api/audios/{nome}", a.handleAudio)

	mux.HandleFunc("GET /api/instrucoes", a.handleInstrucoes)
	mux.HandleFunc("GET /api/instrucoes/{arquivo}", a.handleInstrucao)
	mux.HandleFunc("PUT /api/instrucoes/{arquivo}", a.handleSalvarInstrucao)
	mux.HandleFunc("DELETE /api/instrucoes/{arquivo}", a.handleRestaurarInstrucao)

	mux.HandleFunc("GET /api/sugestoes", a.handleSugestoes)
	mux.HandleFunc("PUT /api/sugestoes/{id}", a.handleGuardarSugestao)
	mux.HandleFunc("PATCH /api/sugestoes/{id}", a.handleMarcarSugestao)
	mux.HandleFunc("DELETE /api/sugestoes/{id}", a.handleApagarSugestao)

	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		erroJSON(w, http.StatusNotFound, "não encontrado")
	})
	mux.Handle("/", spaHandler(web))
	return mux
}

func (a *app) handleHealth(version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := a.db.PingContext(r.Context()); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "erro", "version": version})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": version})
	}
}

// ---------- Utilidades ----------

// spaHandler entrega os arquivos do frontend; caminhos desconhecidos recebem o index.html.
func spaHandler(web fs.FS) http.Handler {
	files := http.FileServerFS(web)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name != "" {
			if _, err := fs.Stat(web, name); err != nil {
				http.ServeFileFS(w, r, web, "index.html")
				return
			}
		}
		files.ServeHTTP(w, r)
	})
}

func lerCorpo(w http.ResponseWriter, r *http.Request, destino any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 5<<20)
	if err := json.NewDecoder(r.Body).Decode(destino); err != nil {
		erroJSON(w, http.StatusBadRequest, "Pedido inválido.")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}

func erroJSON(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"erro": msg})
}

func falha(w http.ResponseWriter, err error) {
	log.Print(err)
	erroJSON(w, http.StatusInternalServerError, err.Error())
}
