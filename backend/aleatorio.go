package main

import "net/http"

// Estudar aleatório (F38): cartões sorteados de um baralho, de um tema ou de todos, a qualquer
// hora, mesmo com a fila do dia por fazer. Só lê: não há nota, não fica revisão no histórico e o
// agendamento do FSRS não muda. Cartões novos entram também; suspensos, não.

const sorteadosPorVez = 30

type cartaoSorteado struct {
	cartaoDeEstudo
	Baralho string `json:"baralho"`
}

// handleAleatorio: GET /api/aleatorio?baralho= (ou ?tema=, ou nada para todos) — uma leva de
// cartões sorteados; o aparelho pede outra quando ela acaba.
func (a *app) handleAleatorio(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filtro, args := ``, []any{}
	switch {
	case q.Get("baralho") != "":
		filtro, args = ` AND c.baralho_id = ?`, []any{q.Get("baralho")}
	case q.Get("tema") != "":
		filtro, args = ` AND b.tema = ?`, []any{q.Get("tema")}
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT c.id, c.nota_id, c.sentido, c.estado, n.tipo, n.frente, n.verso, n.tags, n.imagem, n.imagem_frente, n.audio, n.fonte, b.nome
		FROM cartoes c JOIN notas n ON n.id = c.nota_id JOIN baralhos b ON b.id = c.baralho_id
		WHERE c.suspenso = 0`+filtro+` ORDER BY RANDOM() LIMIT ?`, append(args, sorteadosPorVez)...)
	if err != nil {
		falha(w, err)
		return
	}
	defer rows.Close()
	lista := []cartaoSorteado{}
	for rows.Next() {
		var c cartaoSorteado
		if err := rows.Scan(&c.ID, &c.NotaID, &c.Sentido, &c.Estado, &c.Tipo, &c.Frente, &c.Verso, &c.Tags, &c.Imagem, &c.ImagemFrente, &c.Audio, &c.Fonte, &c.Baralho); err != nil {
			falha(w, err)
			return
		}
		if c.Sentido == 1 {
			c.Frente, c.Verso = c.Verso, c.Frente
			c.Imagem, c.ImagemFrente = c.ImagemFrente, c.Imagem
		}
		lista = append(lista, c)
	}
	if err := rows.Err(); err != nil {
		falha(w, err)
		return
	}
	writeJSON(w, http.StatusOK, lista)
}
