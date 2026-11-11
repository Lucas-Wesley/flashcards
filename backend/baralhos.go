package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
)

const colunasBaralho = `id, nome, tema, novos_por_dia, revisoes_por_dia, retencao, criado_em, atualizado_em`

func lerBaralho(l linha) (baralho, error) {
	var b baralho
	err := l.Scan(&b.ID, &b.Nome, &b.Tema, &b.NovosPorDia, &b.RevisoesPorDia, &b.Retencao, &b.CriadoEm, &b.AtualizadoEm)
	return b, err
}

func (a *app) baralho(ctx context.Context, id string) (baralho, error) {
	return lerBaralho(a.db.QueryRowContext(ctx, `SELECT `+colunasBaralho+` FROM baralhos WHERE id = ?`, id))
}

func (a *app) baralhoDoPedido(w http.ResponseWriter, r *http.Request) (baralho, bool) {
	b, err := a.baralho(r.Context(), r.PathValue("id"))
	if errors.Is(err, sql.ErrNoRows) {
		erroJSON(w, http.StatusNotFound, "Baralho não encontrado.")
		return b, false
	}
	if err != nil {
		falha(w, err)
		return b, false
	}
	return b, true
}

// resumo: o baralho com o que há para estudar hoje e o tamanho dele.
type resumo struct {
	baralho
	Contagem   contagem `json:"contagem"`
	FeitosHoje int      `json:"feitos_hoje"`
	Cartoes    int      `json:"cartoes"`
	Suspensos  int      `json:"suspensos"`
}

func (a *app) resumir(ctx context.Context, b baralho) (resumo, error) {
	f, err := a.montarFila(ctx, b)
	if err != nil {
		return resumo{}, err
	}
	s := resumo{baralho: b, FeitosHoje: f.feitosHoje}
	if s.Contagem, err = a.contagem(ctx, f); err != nil {
		return s, err
	}
	err = a.db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(suspenso), 0) FROM cartoes WHERE baralho_id = ?`, b.ID).Scan(&s.Cartoes, &s.Suspensos)
	return s, err
}

func (a *app) handleBaralhos(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.QueryContext(r.Context(), `SELECT `+colunasBaralho+` FROM baralhos ORDER BY nome COLLATE NOCASE`)
	if err != nil {
		falha(w, err)
		return
	}
	var lista []baralho
	for rows.Next() {
		b, err := lerBaralho(rows)
		if err != nil {
			rows.Close()
			falha(w, err)
			return
		}
		lista = append(lista, b)
	}
	rows.Close()
	saida := make([]resumo, 0, len(lista))
	for _, b := range lista {
		s, err := a.resumir(r.Context(), b)
		if err != nil {
			falha(w, err)
			return
		}
		saida = append(saida, s)
	}
	writeJSON(w, http.StatusOK, saida)
}

func (a *app) handleBaralho(w http.ResponseWriter, r *http.Request) {
	b, ok := a.baralhoDoPedido(w, r)
	if !ok {
		return
	}
	s, err := a.resumir(r.Context(), b)
	if err != nil {
		falha(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s)
}

func (a *app) handleCriarBaralho(w http.ResponseWriter, r *http.Request) {
	var corpo struct {
		Nome string `json:"nome"`
		Tema string `json:"tema"`
	}
	if !lerCorpo(w, r, &corpo) {
		return
	}
	nome := strings.TrimSpace(corpo.Nome)
	if nome == "" {
		erroJSON(w, http.StatusBadRequest, "Dê um nome ao baralho.")
		return
	}
	id := novoID()
	if _, err := a.db.ExecContext(r.Context(), `INSERT INTO baralhos (id, nome, tema, criado_em, atualizado_em) VALUES (?, ?, ?, ?, ?)`,
		id, nome, strings.TrimSpace(corpo.Tema), a.agora(), a.agora()); err != nil {
		falha(w, err)
		return
	}
	b, err := a.baralho(r.Context(), id)
	if err != nil {
		falha(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, b)
}

// handleMudarBaralho muda o nome ou as opções; o que não vier no pedido fica como está.
func (a *app) handleMudarBaralho(w http.ResponseWriter, r *http.Request) {
	b, ok := a.baralhoDoPedido(w, r)
	if !ok {
		return
	}
	var corpo struct {
		Nome           *string  `json:"nome"`
		Tema           *string  `json:"tema"`
		NovosPorDia    *int     `json:"novos_por_dia"`
		RevisoesPorDia *int     `json:"revisoes_por_dia"`
		Retencao       *float64 `json:"retencao"`
	}
	if !lerCorpo(w, r, &corpo) {
		return
	}
	if corpo.Nome != nil {
		b.Nome = strings.TrimSpace(*corpo.Nome)
	}
	if corpo.Tema != nil {
		b.Tema = strings.TrimSpace(*corpo.Tema)
	}
	if corpo.NovosPorDia != nil {
		b.NovosPorDia = *corpo.NovosPorDia
	}
	if corpo.RevisoesPorDia != nil {
		b.RevisoesPorDia = *corpo.RevisoesPorDia
	}
	if corpo.Retencao != nil {
		b.Retencao = *corpo.Retencao
	}
	switch {
	case b.Nome == "":
		erroJSON(w, http.StatusBadRequest, "Dê um nome ao baralho.")
		return
	case b.NovosPorDia < 0 || b.NovosPorDia > 9999 || b.RevisoesPorDia < 0 || b.RevisoesPorDia > 9999:
		erroJSON(w, http.StatusBadRequest, "Os limites diários vão de 0 a 9999.")
		return
	// Abaixo de 70% o cartão é esquecido demais para valer a revisão; acima de 99% as revisões explodem.
	case b.Retencao < 0.7 || b.Retencao > 0.99:
		erroJSON(w, http.StatusBadRequest, "A retenção desejada vai de 70% a 99%.")
		return
	}
	if _, err := a.db.ExecContext(r.Context(), `UPDATE baralhos SET nome = ?, tema = ?, novos_por_dia = ?, revisoes_por_dia = ?, retencao = ?, atualizado_em = ?
		WHERE id = ?`, b.Nome, b.Tema, b.NovosPorDia, b.RevisoesPorDia, b.Retencao, a.agora(), b.ID); err != nil {
		falha(w, err)
		return
	}
	a.handleBaralho(w, r)
}

func (a *app) handleApagarBaralho(w http.ResponseWriter, r *http.Request) {
	if _, err := a.db.ExecContext(r.Context(), `DELETE FROM baralhos WHERE id = ?`, r.PathValue("id")); err != nil {
		falha(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
