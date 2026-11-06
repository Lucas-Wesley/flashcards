package main

import (
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

// Uma nota é o conteúdo (frente, verso, tags); os cartões são o que se estuda. A nota "básica" dá
// um cartão; a "invertida" dá dois, um em cada sentido, para idiomas (casa → house e house → casa).
type nota struct {
	ID           string         `json:"id"`
	BaralhoID    string         `json:"baralho_id"`
	Tipo         string         `json:"tipo"`
	Frente       string         `json:"frente"`
	Verso        string         `json:"verso"`
	Tags         string         `json:"tags"`
	Imagem       string         `json:"imagem"`
	CriadaEm     string         `json:"criada_em"`
	AtualizadaEm string         `json:"atualizada_em"`
	Cartoes      []cartaoResumo `json:"cartoes"`
}

type cartaoResumo struct {
	ID            string `json:"id"`
	Sentido       int    `json:"sentido"`
	Estado        int    `json:"estado"`
	VenceEm       string `json:"vence_em"`
	DiasAgendados int64  `json:"dias_agendados"`
	Lapsos        int64  `json:"lapsos"`
	Suspenso      bool   `json:"suspenso"`
}

var tiposDeNota = map[string]int{"basico": 1, "invertido": 2}

// limparTags aceita tags separadas por espaço ou vírgula e tira as repetidas.
func limparTags(s string) string {
	vistas := map[string]bool{}
	var saida []string
	for _, t := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' || r == '\n' || r == '\t' }) {
		if !vistas[strings.ToLower(t)] {
			vistas[strings.ToLower(t)] = true
			saida = append(saida, t)
		}
	}
	return strings.Join(saida, " ")
}

type dadosDaNota struct {
	BaralhoID string `json:"baralho_id"`
	Tipo      string `json:"tipo"`
	Frente    string `json:"frente"`
	Verso     string `json:"verso"`
	Tags      string `json:"tags"`
	Imagem    string `json:"imagem"`
}

func (d *dadosDaNota) validar() string {
	d.Frente = strings.TrimSpace(d.Frente)
	d.Verso = strings.TrimSpace(d.Verso)
	d.Tags = limparTags(d.Tags)
	if d.Tipo == "" {
		d.Tipo = "basico"
	}
	switch {
	case tiposDeNota[d.Tipo] == 0:
		return "Tipo de nota inválido."
	case d.Frente == "":
		return "Escreva a frente do cartão."
	case d.Tipo == "invertido" && d.Verso == "":
		return "No cartão nos dois sentidos, o verso também é pergunta: escreva o verso."
	case d.Imagem != "" && !nomeDeImagem.MatchString(d.Imagem):
		return "Imagem inválida."
	}
	return ""
}

// criarCartoes cria os cartões que faltam para o tipo da nota. Cartão novo vai para o fim da fila.
func criarCartoes(ctx context.Context, tx *sql.Tx, notaID, baralhoID, tipo, agora string) error {
	for sentido := range tiposDeNota[tipo] {
		// O cartão que já existe (mesma nota e sentido) fica como está, com o histórico.
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO cartoes (id, nota_id, baralho_id, sentido, posicao, vence_em)
			VALUES (?, ?, ?, ?, (SELECT COALESCE(MAX(posicao), 0) + 1 FROM cartoes), ?)`,
			novoID(), notaID, baralhoID, sentido, agora); err != nil {
			return err
		}
	}
	return nil
}

func (a *app) criarNota(ctx context.Context, tx *sql.Tx, d dadosDaNota) (string, error) {
	id := novoID()
	if _, err := tx.ExecContext(ctx, `INSERT INTO notas (id, baralho_id, tipo, frente, verso, tags, imagem, criada_em, atualizada_em)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, id, d.BaralhoID, d.Tipo, d.Frente, d.Verso, d.Tags, d.Imagem, a.agora(), a.agora()); err != nil {
		return "", err
	}
	return id, criarCartoes(ctx, tx, id, d.BaralhoID, d.Tipo, a.agora())
}

func (a *app) existeBaralho(ctx context.Context, id string) bool {
	var n int
	a.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM baralhos WHERE id = ?`, id).Scan(&n)
	return n > 0
}

func (a *app) handleCriarNota(w http.ResponseWriter, r *http.Request) {
	var d dadosDaNota
	if !lerCorpo(w, r, &d) {
		return
	}
	if msg := d.validar(); msg != "" {
		erroJSON(w, http.StatusBadRequest, msg)
		return
	}
	ctx := r.Context()
	if !a.existeBaralho(ctx, d.BaralhoID) {
		erroJSON(w, http.StatusBadRequest, "Escolha um baralho.")
		return
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		falha(w, err)
		return
	}
	defer tx.Rollback()
	id, err := a.criarNota(ctx, tx, d)
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		falha(w, err)
		return
	}
	a.responderNota(w, r, http.StatusCreated, id)
}

func (a *app) handleMudarNota(w http.ResponseWriter, r *http.Request) {
	var d dadosDaNota
	if !lerCorpo(w, r, &d) {
		return
	}
	if msg := d.validar(); msg != "" {
		erroJSON(w, http.StatusBadRequest, msg)
		return
	}
	ctx := r.Context()
	if !a.existeBaralho(ctx, d.BaralhoID) {
		erroJSON(w, http.StatusBadRequest, "Escolha um baralho.")
		return
	}
	id := r.PathValue("id")
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		falha(w, err)
		return
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `UPDATE notas SET baralho_id = ?, tipo = ?, frente = ?, verso = ?, tags = ?, imagem = ?, atualizada_em = ? WHERE id = ?`,
		d.BaralhoID, d.Tipo, d.Frente, d.Verso, d.Tags, d.Imagem, a.agora(), id)
	if err != nil {
		falha(w, err)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		erroJSON(w, http.StatusNotFound, "Nota não encontrada.")
		return
	}
	// Mudar de baralho leva os cartões com o histórico; virar básica apaga o cartão do verso.
	_, err = tx.ExecContext(ctx, `UPDATE cartoes SET baralho_id = ? WHERE nota_id = ?`, d.BaralhoID, id)
	if err == nil {
		_, err = tx.ExecContext(ctx, `DELETE FROM cartoes WHERE nota_id = ? AND sentido >= ?`, id, tiposDeNota[d.Tipo])
	}
	if err == nil {
		err = criarCartoes(ctx, tx, id, d.BaralhoID, d.Tipo, a.agora())
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		falha(w, err)
		return
	}
	a.responderNota(w, r, http.StatusOK, id)
}

func (a *app) handleApagarNota(w http.ResponseWriter, r *http.Request) {
	if _, err := a.db.ExecContext(r.Context(), `DELETE FROM notas WHERE id = ?`, r.PathValue("id")); err != nil {
		falha(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *app) handleNota(w http.ResponseWriter, r *http.Request) {
	a.responderNota(w, r, http.StatusOK, r.PathValue("id"))
}

func (a *app) responderNota(w http.ResponseWriter, r *http.Request, status int, id string) {
	lista, err := a.notas(r.Context(), `WHERE id = ?`, id)
	if err != nil {
		falha(w, err)
		return
	}
	if len(lista) == 0 {
		erroJSON(w, http.StatusNotFound, "Nota não encontrada.")
		return
	}
	writeJSON(w, status, lista[0])
}

// handleNotas lista as notas, mais novas primeiro, com busca na frente, no verso e nas tags.
func (a *app) handleNotas(w http.ResponseWriter, r *http.Request) {
	where := []string{"1 = 1"}
	var args []any
	if b := r.URL.Query().Get("baralho"); b != "" {
		where = append(where, "baralho_id = ?")
		args = append(args, b)
	}
	for _, termo := range strings.Fields(r.URL.Query().Get("q")) {
		where = append(where, `(frente LIKE ? ESCAPE '\' OR verso LIKE ? ESCAPE '\' OR tags LIKE ? ESCAPE '\')`)
		t := "%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(termo) + "%"
		args = append(args, t, t, t)
	}
	limite, _ := strconv.Atoi(r.URL.Query().Get("limite"))
	if limite <= 0 || limite > 1000 {
		limite = 300
	}
	lista, err := a.notas(r.Context(), `WHERE `+strings.Join(where, " AND ")+` ORDER BY criada_em DESC, rowid DESC LIMIT `+strconv.Itoa(limite), args...)
	if err != nil {
		falha(w, err)
		return
	}
	writeJSON(w, http.StatusOK, lista)
}

func (a *app) notas(ctx context.Context, filtro string, args ...any) ([]nota, error) {
	rows, err := a.db.QueryContext(ctx, `SELECT id, baralho_id, tipo, frente, verso, tags, imagem, criada_em, atualizada_em FROM notas `+filtro, args...)
	if err != nil {
		return nil, err
	}
	lista := []nota{}
	indice := map[string]int{}
	for rows.Next() {
		var n nota
		if err := rows.Scan(&n.ID, &n.BaralhoID, &n.Tipo, &n.Frente, &n.Verso, &n.Tags, &n.Imagem, &n.CriadaEm, &n.AtualizadaEm); err != nil {
			rows.Close()
			return nil, err
		}
		n.Cartoes = []cartaoResumo{}
		indice[n.ID] = len(lista)
		lista = append(lista, n)
	}
	rows.Close()
	if err := rows.Err(); err != nil || len(lista) == 0 {
		return lista, err
	}

	ids := make([]any, 0, len(lista))
	for _, n := range lista {
		ids = append(ids, n.ID)
	}
	rows, err = a.db.QueryContext(ctx, `SELECT nota_id, id, sentido, estado, vence_em, dias_agendados, lapsos, suspenso FROM cartoes
		WHERE nota_id IN (?`+strings.Repeat(", ?", len(ids)-1)+`) ORDER BY sentido`, ids...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var notaID string
		var c cartaoResumo
		if err := rows.Scan(&notaID, &c.ID, &c.Sentido, &c.Estado, &c.VenceEm, &c.DiasAgendados, &c.Lapsos, &c.Suspenso); err != nil {
			return nil, err
		}
		i := indice[notaID]
		lista[i].Cartoes = append(lista[i].Cartoes, c)
	}
	return lista, rows.Err()
}

// handleMudarCartao suspende ou reativa um cartão: suspenso, ele sai da fila sem perder o histórico.
func (a *app) handleMudarCartao(w http.ResponseWriter, r *http.Request) {
	var corpo struct {
		Suspenso bool `json:"suspenso"`
	}
	if !lerCorpo(w, r, &corpo) {
		return
	}
	res, err := a.db.ExecContext(r.Context(), `UPDATE cartoes SET suspenso = ? WHERE id = ?`, corpo.Suspenso, r.PathValue("id"))
	if err != nil {
		falha(w, err)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		erroJSON(w, http.StatusNotFound, "Cartão não encontrado.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleImportar recebe linhas "frente;verso;tags" (ou com tabulação, ou vírgula), como sai de
// uma planilha ou do "Exportar notas em texto" do Anki. Frente repetida no baralho é pulada.
func (a *app) handleImportar(w http.ResponseWriter, r *http.Request) {
	b, ok := a.baralhoDoPedido(w, r)
	if !ok {
		return
	}
	var corpo struct {
		Texto string `json:"texto"`
		Tipo  string `json:"tipo"`
	}
	if !lerCorpo(w, r, &corpo) {
		return
	}
	linhas, err := lerPlanilha(corpo.Texto)
	if err != nil {
		erroJSON(w, http.StatusBadRequest, "Não consegui ler o texto: "+err.Error())
		return
	}
	ctx := r.Context()
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		falha(w, err)
		return
	}
	defer tx.Rollback()

	existentes := map[string]bool{}
	rows, err := tx.QueryContext(ctx, `SELECT frente FROM notas WHERE baralho_id = ?`, b.ID)
	if err != nil {
		falha(w, err)
		return
	}
	for rows.Next() {
		var f string
		rows.Scan(&f)
		existentes[f] = true
	}
	rows.Close()

	var resultado struct {
		Importadas int `json:"importadas"`
		Repetidas  int `json:"repetidas"`
		Ignoradas  int `json:"ignoradas"`
	}
	for _, campos := range linhas {
		d := dadosDaNota{BaralhoID: b.ID, Tipo: corpo.Tipo}
		if len(campos) > 0 {
			d.Frente = campos[0]
		}
		if len(campos) > 1 {
			d.Verso = campos[1]
		}
		if len(campos) > 2 {
			d.Tags = campos[2]
		}
		if d.validar() != "" || d.Verso == "" {
			resultado.Ignoradas++
			continue
		}
		if existentes[d.Frente] {
			resultado.Repetidas++
			continue
		}
		existentes[d.Frente] = true
		if _, err := a.criarNota(ctx, tx, d); err != nil {
			falha(w, err)
			return
		}
		resultado.Importadas++
	}
	if err := tx.Commit(); err != nil {
		falha(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resultado)
}

// lerPlanilha descobre o separador pela primeira linha com conteúdo: tabulação, ponto e vírgula
// ou vírgula, nessa ordem. Linhas começando com "#" são comentário (cabeçalho do Anki).
func lerPlanilha(texto string) ([][]string, error) {
	var uteis []string
	for _, l := range strings.Split(strings.ReplaceAll(texto, "\r\n", "\n"), "\n") {
		if strings.TrimSpace(l) != "" && !strings.HasPrefix(l, "#") {
			uteis = append(uteis, l)
		}
	}
	if len(uteis) == 0 {
		return nil, errors.New("nenhuma linha com conteúdo")
	}
	sep := ','
	for _, s := range []rune{'\t', ';'} {
		if strings.ContainsRune(uteis[0], s) {
			sep = s
			break
		}
	}
	leitor := csv.NewReader(strings.NewReader(strings.Join(uteis, "\n")))
	leitor.Comma = sep
	leitor.FieldsPerRecord = -1
	leitor.LazyQuotes = true
	return leitor.ReadAll()
}
