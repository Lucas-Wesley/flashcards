package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// Propostas prontas de outro app (F45). O Streaming escolhe as frases de um filme pela legenda e
// manda os cartões já montados (a frase em inglês na frente; a tradução e a expressão explicada no
// verso). Não passam pela IA daqui: viram um pedido já em revisão, como os outros, e só entram no
// baralho depois de o Lucas escutar e aprovar.
//
// O áudio vem depois: o Streaming corta cada fala do vídeo quando pode (com a conexão da assinatura
// livre) e manda por POST /api/geracoes/{id}/audios. Cada cartão é identificado pela fonte (o link
// da fala no Streaming, único por frase), que a nota guarda ao ser aprovada: o áudio que chega
// depois da aprovação vai direto para a nota.

const origemStreaming = "streaming"

// maximoDeCartoesProntos: um filme rende no máximo algumas dezenas de frases boas.
const maximoDeCartoesProntos = 200

func (a *app) handleProposta(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	var corpo struct {
		Origem      string           `json:"origem"`
		Titulo      string           `json:"titulo"`     // "Frases de O Convite"
		Link        string           `json:"link"`       // o título no Streaming
		BaralhoID   string           `json:"baralho_id"` // o baralho que já existe
		Contexto    string           `json:"contexto"`   // o que aparece embaixo do título na revisão
		Observacoes string           `json:"observacoes"`
		Cartoes     []cartaoProposto `json:"cartoes"`
	}
	if !lerCorpo(w, r, &corpo) {
		return
	}
	corpo.Titulo, corpo.Link = strings.TrimSpace(corpo.Titulo), strings.TrimSpace(corpo.Link)
	switch {
	case corpo.Origem != origemStreaming:
		erroJSON(w, http.StatusBadRequest, "Origem inválida.")
		return
	case corpo.Titulo == "":
		erroJSON(w, http.StatusBadRequest, "Diga de onde vêm os cartões.")
		return
	case !linkDeFonte(corpo.Link):
		erroJSON(w, http.StatusBadRequest, "Falta o link de volta.")
		return
	case corpo.BaralhoID == "" || !a.existeBaralho(r.Context(), corpo.BaralhoID):
		erroJSON(w, http.StatusBadRequest, "Baralho não encontrado.")
		return
	case len(corpo.Cartoes) == 0 || len(corpo.Cartoes) > maximoDeCartoesProntos:
		erroJSON(w, http.StatusBadRequest, fmt.Sprintf("Mande de 1 a %d cartões.", maximoDeCartoesProntos))
		return
	}
	vistas := map[string]bool{}
	cartoes := make([]cartaoProposto, 0, len(corpo.Cartoes))
	for _, c := range corpo.Cartoes {
		c.Frente, c.Verso, c.Tags, c.Fonte = strings.TrimSpace(c.Frente), strings.TrimSpace(c.Verso), limparTags(c.Tags), strings.TrimSpace(c.Fonte)
		if c.Frente == "" || c.Verso == "" || utf8.RuneCountInString(c.Frente) > 1000 || utf8.RuneCountInString(c.Verso) > 3000 {
			erroJSON(w, http.StatusBadRequest, "Todo cartão precisa de frente e verso: “"+cortar(c.Frente, 40)+"”.")
			return
		}
		if !linkDeFonte(c.Fonte) || vistas[c.Fonte] {
			erroJSON(w, http.StatusBadRequest, "Cada cartão precisa de uma fonte própria (o link da fala).")
			return
		}
		vistas[c.Fonte] = true
		// Só o áudio que já está aqui (mandado antes por POST /api/audios).
		if nome, ok := strings.CutPrefix(c.AudioURL, "/api/audios/"); !ok || !a.existeAudio(nome) {
			c.AudioURL = ""
		} else {
			c.AudioPendente = false
		}
		c.ImagemURL, c.ImagemFrenteURL, c.Imagem, c.ImagemFrente, c.Atualiza, c.Secao, c.Avisos = "", "", "", "", false, "", nil
		cartoes = append(cartoes, c)
	}
	p := proposta{Observacoes: strings.TrimSpace(corpo.Observacoes), Cartoes: cartoes}
	b, _ := json.Marshal(p)
	ctx := r.Context()

	// Mandar de novo o mesmo título, ainda em revisão, troca a proposta (não cria outra).
	var id int64
	err := a.db.QueryRowContext(ctx, `SELECT id FROM geracoes WHERE origem = ? AND link = ? AND estado = ? ORDER BY id DESC LIMIT 1`,
		corpo.Origem, corpo.Link, emRevisao).Scan(&id)
	switch {
	case err == nil:
		_, err = a.db.ExecContext(ctx, `UPDATE geracoes SET pedido = ?, contexto = ?, baralho_id = ?, proposta = ?, quantidade = ?, terminado_em = ? WHERE id = ?`,
			corpo.Titulo, strings.TrimSpace(corpo.Contexto), corpo.BaralhoID, string(b), len(cartoes), a.agora(), id)
	case errors.Is(err, sql.ErrNoRows):
		var res sql.Result
		res, err = a.db.ExecContext(ctx, `INSERT INTO geracoes (origem, pedido, contexto, link, quantidade, tipo, baralho_id, estado, proposta, criado_em, iniciado_em, terminado_em)
			VALUES (?, ?, ?, ?, ?, 'basico', ?, ?, ?, ?, ?, ?)`,
			corpo.Origem, corpo.Titulo, strings.TrimSpace(corpo.Contexto), corpo.Link, len(cartoes), corpo.BaralhoID, emRevisao, string(b), a.agora(), a.agora(), a.agora())
		if err == nil {
			id, _ = res.LastInsertId()
		}
	}
	if err != nil {
		falha(w, err)
		return
	}
	if g, err := a.geracao(ctx, id); err == nil {
		a.avisarGeracao(g, "")
	}
	a.responderGeracao(w, r, http.StatusCreated, id)
}

func (a *app) existeAudio(nome string) bool {
	if !nomeDeAudio.MatchString(nome) {
		return false
	}
	_, err := os.Stat(filepath.Join(a.cfg.dados, "media", nome))
	return err == nil
}

// audioDoCartao: o áudio de uma fala, pela fonte. Sem Audio, a fala ficou sem áudio de vez (não deu
// para cortar): o cartão deixa de esperar.
type audioDoCartao struct {
	Fonte string `json:"fonte"`
	Audio string `json:"audio"`
}

// handleAudiosDaProposta: POST /api/geracoes/{id}/audios. Na proposta em revisão, o áudio entra no
// cartão; aprovada, na nota de mesma fonte que ainda não tem áudio.
func (a *app) handleAudiosDaProposta(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaGeracao(w, r)
	if !ok {
		return
	}
	var corpo struct {
		Audios []audioDoCartao `json:"audios"`
	}
	if !lerCorpo(w, r, &corpo) {
		return
	}
	for _, au := range corpo.Audios {
		if au.Audio != "" && !a.existeAudio(au.Audio) {
			erroJSON(w, http.StatusBadRequest, "Áudio não encontrado: mande antes por POST /api/audios.")
			return
		}
	}
	n, err := a.guardarAudiosDaProposta(r.Context(), id, corpo.Audios)
	if errors.Is(err, sql.ErrNoRows) {
		erroJSON(w, http.StatusNotFound, "Pedido não encontrado.")
		return
	}
	if err != nil {
		falha(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"guardados": n})
}

func (a *app) guardarAudiosDaProposta(ctx context.Context, id int64, audios []audioDoCartao) (int, error) {
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	g, err := lerGeracao(tx.QueryRowContext(ctx, `SELECT `+colunasGeracao+` FROM geracoes WHERE id = ?`, id))
	if err != nil {
		return 0, err
	}
	porFonte := map[string]string{}
	for _, au := range audios {
		porFonte[strings.TrimSpace(au.Fonte)] = au.Audio
	}
	guardados := 0
	if g.Proposta != nil {
		for i, c := range g.Proposta.Cartoes {
			audio, veio := porFonte[c.Fonte]
			if !veio || c.Fonte == "" {
				continue
			}
			g.Proposta.Cartoes[i].AudioPendente = false
			if audio != "" {
				g.Proposta.Cartoes[i].AudioURL = "/api/audios/" + audio
				guardados++
			}
		}
		b, _ := json.Marshal(g.Proposta)
		if _, err := tx.ExecContext(ctx, `UPDATE geracoes SET proposta = ? WHERE id = ?`, string(b), id); err != nil {
			return 0, err
		}
	}
	if g.Estado == concluida && g.Resultado != "" {
		guardados = 0
		for fonte, audio := range porFonte {
			if audio == "" || fonte == "" {
				continue
			}
			res, err := tx.ExecContext(ctx, `UPDATE notas SET audio = ?, atualizada_em = ? WHERE baralho_id = ? AND fonte = ? AND audio = ''`,
				audio, a.agora(), g.Resultado, fonte)
			if err != nil {
				return 0, err
			}
			n, _ := res.RowsAffected()
			guardados += int(n)
		}
	}
	return guardados, tx.Commit()
}

// audiosQueChegaram: na aprovação, o cartão que a tela ainda via esperando o áudio leva o que
// chegou enquanto o Lucas revisava.
func audiosQueChegaram(guardada *proposta, cartoes []cartaoProposto) {
	if guardada == nil {
		return
	}
	porFonte := map[string]string{}
	for _, c := range guardada.Cartoes {
		if c.Fonte != "" && c.AudioURL != "" {
			porFonte[c.Fonte] = c.AudioURL
		}
	}
	for i, c := range cartoes {
		if c.AudioPendente && c.AudioURL == "" {
			cartoes[i].AudioURL = porFonte[c.Fonte]
		}
	}
}
