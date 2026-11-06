package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/open-spaced-repetition/go-fsrs/v4"
)

// A fila de estudo de um baralho, na ordem do Anki:
//  1. cartões aprendendo (ou reaprendendo) que já venceram, que esperam minutos, não dias;
//  2. revisões que vencem até o fim do dia de estudo, até o limite diário;
//  3. cartões novos, na ordem em que entraram, até o limite diário;
//  4. sem mais nada, os aprendendo que vencem nos próximos 20 minutos, para não ficar parado.
//
// Irmãos (o outro sentido da mesma nota) ficam para o dia seguinte quando um deles já foi estudado
// hoje: ver "casa → house" e logo depois "house → casa" não treina nada.
const aprenderAdiante = 20 * time.Minute

type contagem struct {
	Novos      int `json:"novos"`
	Aprendendo int `json:"aprendendo"`
	Revisar    int `json:"revisar"`
}

type fila struct {
	b                  baralho
	agora, inicio, fim time.Time
	restamNovos        int
	restamRevisoes     int
	feitosHoje         int
}

const (
	doBaralho  = `baralho_id = ? AND suspenso = 0`
	semIrmaos  = ` AND nota_id NOT IN (SELECT c.nota_id FROM revisoes r JOIN cartoes c ON c.id = r.cartao_id WHERE r.baralho_id = ? AND r.revisado_em >= ?)`
	emAprender = ` AND estado IN (1, 3) AND vence_em <= ?`
	aRevisar   = ` AND estado = 2 AND vence_em < ?`
	novos      = ` AND estado = 0`
)

func (a *app) montarFila(ctx context.Context, b baralho) (fila, error) {
	f := fila{b: b, agora: a.rel.agora()}
	f.inicio, f.fim = a.rel.dia(f.agora)
	var novosHoje, revisoesHoje int
	err := a.db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(estado_antes = 0), 0), COALESCE(SUM(estado_antes = 2), 0)
		FROM revisoes WHERE baralho_id = ? AND revisado_em >= ?`, b.ID, hora(f.inicio)).Scan(&f.feitosHoje, &novosHoje, &revisoesHoje)
	f.restamNovos = max(0, b.NovosPorDia-novosHoje)
	f.restamRevisoes = max(0, b.RevisoesPorDia-revisoesHoje)
	return f, err
}

func (a *app) contar(ctx context.Context, f fila, filtro string, args ...any) (int, error) {
	var n int
	err := a.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM cartoes WHERE `+doBaralho+filtro, append([]any{f.b.ID}, args...)...).Scan(&n)
	return n, err
}

func (a *app) contagem(ctx context.Context, f fila) (contagem, error) {
	var c contagem
	var err error
	irmaos := []any{f.b.ID, hora(f.inicio)}
	if c.Aprendendo, err = a.contar(ctx, f, emAprender, hora(f.agora.Add(aprenderAdiante))); err != nil {
		return c, err
	}
	if c.Revisar, err = a.contar(ctx, f, aRevisar+semIrmaos, append([]any{hora(f.fim)}, irmaos...)...); err != nil {
		return c, err
	}
	if c.Novos, err = a.contar(ctx, f, novos+semIrmaos, irmaos...); err != nil {
		return c, err
	}
	c.Revisar = min(c.Revisar, f.restamRevisoes)
	c.Novos = min(c.Novos, f.restamNovos)
	return c, nil
}

// proximo devolve o cartão da vez, ou nil quando não há nada para agora.
func (a *app) proximo(ctx context.Context, f fila) (*cartao, error) {
	irmaos := []any{f.b.ID, hora(f.inicio)}
	type passo struct {
		ativo  bool
		filtro string
		args   []any
	}
	passos := []passo{
		{true, emAprender + ` ORDER BY vence_em`, []any{hora(f.agora)}},
		{f.restamRevisoes > 0, aRevisar + semIrmaos + ` ORDER BY vence_em`, append([]any{hora(f.fim)}, irmaos...)},
		{f.restamNovos > 0, novos + semIrmaos + ` ORDER BY posicao`, irmaos},
		{true, emAprender + ` ORDER BY vence_em`, []any{hora(f.agora.Add(aprenderAdiante))}},
	}
	for _, p := range passos {
		if !p.ativo {
			continue
		}
		c, err := lerCartao(a.db.QueryRowContext(ctx, `SELECT `+colunasCartao+` FROM cartoes WHERE `+doBaralho+p.filtro+` LIMIT 1`,
			append([]any{f.b.ID}, p.args...)...))
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, err
		}
		return &c, nil
	}
	return nil, nil
}

type cartaoDeEstudo struct {
	ID      string `json:"id"`
	NotaID  string `json:"nota_id"`
	Frente  string `json:"frente"`
	Verso   string `json:"verso"`
	Tags    string `json:"tags"`
	Sentido int    `json:"sentido"`
	Estado  int    `json:"estado"`
	// Repeticoes vai e volta na resposta: um toque duplo não responde o mesmo cartão duas vezes.
	Repeticoes int64 `json:"repeticoes"`
}

type estudo struct {
	Baralho    baralho         `json:"baralho"`
	Contagem   contagem        `json:"contagem"`
	FeitosHoje int             `json:"feitos_hoje"`
	Cartao     *cartaoDeEstudo `json:"cartao"`
	Previsoes  map[int]int64   `json:"previsoes,omitempty"`
	// Sem cartão para agora, mas com algum aprendendo mais tarde hoje: a hora em que ele vence.
	EsperaAte string `json:"espera_ate,omitempty"`
}

// montarEstudo monta a tela de estudo. Com "forcar", mostra aquele cartão (o que acabou de ser
// desfeito) em vez do próximo da fila.
func (a *app) montarEstudo(ctx context.Context, b baralho, forcar *cartao) (estudo, error) {
	f, err := a.montarFila(ctx, b)
	if err != nil {
		return estudo{}, err
	}
	e := estudo{Baralho: b, FeitosHoje: f.feitosHoje}
	if e.Contagem, err = a.contagem(ctx, f); err != nil {
		return e, err
	}
	c := forcar
	if c == nil {
		if c, err = a.proximo(ctx, f); err != nil {
			return e, err
		}
	}
	if c == nil {
		var espera sql.NullString
		a.db.QueryRowContext(ctx, `SELECT MIN(vence_em) FROM cartoes WHERE `+doBaralho+` AND estado IN (1, 3) AND vence_em < ?`,
			b.ID, hora(f.fim)).Scan(&espera)
		e.EsperaAte = espera.String
		return e, nil
	}
	var frente, verso, tags string
	if err := a.db.QueryRowContext(ctx, `SELECT frente, verso, tags FROM notas WHERE id = ?`, c.NotaID).Scan(&frente, &verso, &tags); err != nil {
		return e, err
	}
	if c.Sentido == 1 {
		frente, verso = verso, frente
	}
	e.Cartao = &cartaoDeEstudo{ID: c.ID, NotaID: c.NotaID, Frente: frente, Verso: verso, Tags: tags, Sentido: c.Sentido, Estado: c.Estado, Repeticoes: c.Repeticoes}
	e.Previsoes, err = a.rel.previsoes(*c, b.Retencao, f.agora)
	return e, err
}

func (a *app) handleEstudo(w http.ResponseWriter, r *http.Request) {
	b, ok := a.baralhoDoPedido(w, r)
	if !ok {
		return
	}
	e, err := a.montarEstudo(r.Context(), b, nil)
	if err != nil {
		falha(w, err)
		return
	}
	writeJSON(w, http.StatusOK, e)
}

// handleResponder aplica a nota e já devolve o próximo cartão do baralho.
func (a *app) handleResponder(w http.ResponseWriter, r *http.Request) {
	var corpo struct {
		Nota       int   `json:"nota"`
		Repeticoes int64 `json:"repeticoes"`
		DuracaoMs  int64 `json:"duracao_ms"`
	}
	if !lerCorpo(w, r, &corpo) {
		return
	}
	if corpo.Nota < 1 || corpo.Nota > 4 {
		erroJSON(w, http.StatusBadRequest, "Nota inválida: use de 1 a 4.")
		return
	}
	ctx := r.Context()
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		falha(w, err)
		return
	}
	defer tx.Rollback()

	c, err := lerCartao(tx.QueryRowContext(ctx, `SELECT `+colunasCartao+` FROM cartoes WHERE id = ?`, r.PathValue("id")))
	if errors.Is(err, sql.ErrNoRows) {
		erroJSON(w, http.StatusNotFound, "Cartão não encontrado.")
		return
	}
	if err != nil {
		falha(w, err)
		return
	}
	if c.Repeticoes != corpo.Repeticoes || c.Suspenso {
		erroJSON(w, http.StatusConflict, "Este cartão já foi respondido.")
		return
	}
	b, err := a.baralho(ctx, c.BaralhoID)
	if err != nil {
		falha(w, err)
		return
	}

	agora := a.rel.agora()
	depois, err := a.rel.responderCartao(c, b.Retencao, fsrs.Rating(corpo.Nota), agora)
	if err != nil {
		falha(w, err)
		return
	}
	antes, _ := json.Marshal(c)
	if err := salvarAgenda(tx, depois); err != nil {
		falha(w, err)
		return
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO revisoes (cartao_id, baralho_id, nota, estado_antes, revisado_em, duracao_ms, antes)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, c.ID, c.BaralhoID, corpo.Nota, c.Estado, hora(agora), max(0, min(corpo.DuracaoMs, 3_600_000)), string(antes)); err != nil {
		falha(w, err)
		return
	}
	if err := tx.Commit(); err != nil {
		falha(w, err)
		return
	}
	e, err := a.montarEstudo(ctx, b, nil)
	if err != nil {
		falha(w, err)
		return
	}
	writeJSON(w, http.StatusOK, e)
}

// handleDesfazer volta a última resposta do baralho e mostra aquele cartão de novo.
func (a *app) handleDesfazer(w http.ResponseWriter, r *http.Request) {
	b, ok := a.baralhoDoPedido(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		falha(w, err)
		return
	}
	defer tx.Rollback()

	var id int64
	var antes string
	err = tx.QueryRowContext(ctx, `SELECT id, antes FROM revisoes WHERE baralho_id = ? ORDER BY id DESC LIMIT 1`, b.ID).Scan(&id, &antes)
	if errors.Is(err, sql.ErrNoRows) {
		erroJSON(w, http.StatusNotFound, "Nada para desfazer.")
		return
	}
	if err != nil {
		falha(w, err)
		return
	}
	var c cartao
	if err := json.Unmarshal([]byte(antes), &c); err != nil {
		falha(w, err)
		return
	}
	if err := salvarAgenda(tx, c); err != nil {
		falha(w, err)
		return
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM revisoes WHERE id = ?`, id); err != nil {
		falha(w, err)
		return
	}
	if err := tx.Commit(); err != nil {
		falha(w, err)
		return
	}
	e, err := a.montarEstudo(ctx, b, &c)
	if err != nil {
		falha(w, err)
		return
	}
	writeJSON(w, http.StatusOK, e)
}
