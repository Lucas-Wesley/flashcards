package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"strings"
	"time"
)

// Falar o pedido (F39): o aparelho grava a fala (o Chrome grava em WebM/Opus) e o servidor manda ao
// Whisper large-v3 do Mac mini, o mesmo do Acervo; com o Mac desligado, ao Whisper do servidor. O
// texto volta para o campo do pedido, onde o Lucas confere antes de mandar à IA.

const (
	tamanhoMaximoDaFala = 25 << 20 // ~25 min de Opus; um pedido falado tem um ou dois
	modeloDoMac         = "whisper-large-v3"
	modeloDaReserva     = "deepdml/faster-whisper-large-v3-turbo-ct2"
	// O Whisper acerta mais os nomes com um contexto.
	contextoDaFala = "Pedido de flashcards para estudar: o assunto, o que vai na frente e no verso de cada cartão."
)

var (
	clienteDaFala     = &http.Client{Timeout: 5 * time.Minute}
	errWhisperFora    = errors.New("o Whisper está fora do ar")
	errSemTranscricao = errors.New("A transcrição não está configurada (WHISPER_URL).")
)

func (a *app) handleTranscrever(w http.ResponseWriter, r *http.Request) {
	if a.cfg.whisper == "" && a.cfg.whisperReserva == "" {
		erroJSON(w, http.StatusServiceUnavailable, errSemTranscricao.Error())
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, tamanhoMaximoDaFala+1<<20)
	arquivo, cab, err := r.FormFile("audio")
	if err != nil {
		erroJSON(w, http.StatusBadRequest, "Envie a gravação (até 25 MB).")
		return
	}
	defer arquivo.Close()
	audio, err := io.ReadAll(arquivo)
	if err != nil || len(audio) == 0 {
		erroJSON(w, http.StatusBadRequest, "A gravação veio vazia.")
		return
	}
	texto, err := a.transcrever(r.Context(), audio, cab.Filename)
	if err != nil {
		erroJSON(w, http.StatusBadGateway, "Não deu para transcrever: "+err.Error())
		return
	}
	if texto == "" {
		erroJSON(w, http.StatusUnprocessableEntity, "Não ouvi nenhuma fala na gravação.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"texto": texto})
}

// transcrever tenta o Mac (se ele responde) e, por falha de rede ou serviço fora, a reserva.
func (a *app) transcrever(ctx context.Context, audio []byte, nome string) (string, error) {
	if a.cfg.whisper != "" && (a.cfg.whisperReserva == "" || a.whisperNoAr(ctx, a.cfg.whisper)) {
		texto, err := pedirAoWhisper(ctx, a.cfg.whisper, modeloDoMac, audio, nome)
		var erroDeRede net.Error
		if err == nil || a.cfg.whisperReserva == "" || ctx.Err() != nil || !(errors.As(err, &erroDeRede) || errors.Is(err, errWhisperFora)) {
			return texto, err
		}
	}
	return pedirAoWhisper(ctx, a.cfg.whisperReserva, modeloDaReserva, audio, nome)
}

// whisperNoAr: o Mac desligado não responde, e a conexão ficaria pendurada; pergunta antes, rápido.
func (a *app) whisperNoAr(ctx context.Context, base string) bool {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/saude", nil)
	if err != nil {
		return false
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	res.Body.Close()
	return res.StatusCode == http.StatusOK
}

func pedirAoWhisper(ctx context.Context, base, modelo string, audio []byte, nome string) (string, error) {
	if nome == "" {
		nome = "fala.webm"
	}
	var corpo bytes.Buffer
	mw := multipart.NewWriter(&corpo)
	parte, _ := mw.CreateFormFile("file", nome)
	parte.Write(audio)
	mw.WriteField("model", modelo)
	mw.WriteField("language", "pt")
	mw.WriteField("response_format", "json")
	mw.WriteField("prompt", contextoDaFala)
	mw.Close()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v1/audio/transcriptions", &corpo)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	res, err := clienteDaFala.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode == http.StatusBadGateway || res.StatusCode == http.StatusServiceUnavailable {
		return "", errWhisperFora
	}
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("o Whisper respondeu %d: %s", res.StatusCode, cortar(strings.TrimSpace(string(b)), 200))
	}
	var r struct {
		Texto string `json:"text"`
	}
	if err := json.Unmarshal(b, &r); err != nil {
		return "", fmt.Errorf("resposta do Whisper ilegível: %v", err)
	}
	return strings.TrimSpace(r.Texto), nil
}
