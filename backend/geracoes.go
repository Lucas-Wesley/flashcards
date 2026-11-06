package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Gerar um baralho com IA: o pedido ("presidentes do Brasil no século 21") entra numa fila, o
// Claude monta os cartões seguindo regras/gerar-baralho.md (editável em Ajustes) e devolve uma
// proposta. Nada entra num baralho antes de o Lucas revisar, editar e aprovar.

// Estados de uma geração.
const (
	naFila    = "na_fila"
	rodando   = "rodando"
	emRevisao = "em_revisao"
	concluida = "concluida"
	falhou    = "falhou"
	cancelada = "cancelada"
	rejeitada = "rejeitada"
)

// A IA pesquisa para conferir os fatos, então uma geração pode levar alguns minutos.
const tempoMaximoDaGeracao = 15 * time.Minute

type geracao struct {
	ID          int64     `json:"id"`
	Origem      string    `json:"origem"`
	Pedido      string    `json:"pedido"`
	Quantidade  int       `json:"quantidade"`
	Tipo        string    `json:"tipo"`
	BaralhoID   string    `json:"baralho_id"`
	Estado      string    `json:"estado"`
	Erro        string    `json:"erro"`
	Proposta    *proposta `json:"proposta,omitempty"`
	Resultado   string    `json:"resultado"`
	CriadoEm    string    `json:"criado_em"`
	IniciadoEm  string    `json:"iniciado_em"`
	TerminadoEm string    `json:"terminado_em"`
}

type proposta struct {
	NomeBaralho string           `json:"nome_baralho"`
	Observacoes string           `json:"observacoes"`
	Cartoes     []cartaoProposto `json:"cartoes"`
}

type cartaoProposto struct {
	Frente string `json:"frente"`
	Verso  string `json:"verso"`
	Tags   string `json:"tags"`
	// Imagem: o título do artigo da Wikipédia de onde tirar a foto, quando a IA acha que ajuda.
	// ImagemURL: a foto encontrada; só é baixada para o servidor quando o Lucas aprova.
	Imagem    string `json:"imagem"`
	ImagemURL string `json:"imagem_url"`
	// AudioURL: a pronúncia da frente, quando a página de um link traz (baixada ao aprovar).
	AudioURL string `json:"audio_url"`
}

const esquemaProposta = `{
  "type": "object",
  "properties": {
    "nome_baralho": {"type": "string"},
    "observacoes": {"type": "string"},
    "cartoes": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "frente": {"type": "string"},
          "verso": {"type": "string"},
          "tags": {"type": "string"},
          "imagem": {"type": "string"}
        },
        "required": ["frente", "verso"]
      }
    }
  },
  "required": ["nome_baralho", "cartoes"]
}`

const colunasGeracao = `id, origem, pedido, quantidade, tipo, baralho_id, estado, erro, proposta, resultado, criado_em, iniciado_em, terminado_em`

func lerGeracao(l linha) (*geracao, error) {
	var g geracao
	var p string
	if err := l.Scan(&g.ID, &g.Origem, &g.Pedido, &g.Quantidade, &g.Tipo, &g.BaralhoID, &g.Estado, &g.Erro, &p, &g.Resultado,
		&g.CriadoEm, &g.IniciadoEm, &g.TerminadoEm); err != nil {
		return nil, err
	}
	if p != "" {
		g.Proposta = &proposta{}
		json.Unmarshal([]byte(p), g.Proposta)
	}
	return &g, nil
}

func (a *app) geracao(ctx context.Context, id int64) (*geracao, error) {
	return lerGeracao(a.db.QueryRowContext(ctx, `SELECT `+colunasGeracao+` FROM geracoes WHERE id = ?`, id))
}

// ---------- Fila ----------

// filaDaIA roda uma geração por vez, como no Acervo: cada uma chama o Claude e leva minutos.
type filaDaIA struct {
	a      *app
	acorda chan struct{}

	mu       sync.Mutex
	atual    int64
	cancelar context.CancelFunc
}

func novaFila(a *app) *filaDaIA {
	return &filaDaIA{a: a, acorda: make(chan struct{}, 1)}
}

// recuperar devolve à fila o que estava rodando quando o app parou.
func (f *filaDaIA) recuperar(ctx context.Context) error {
	_, err := f.a.db.ExecContext(ctx, `UPDATE geracoes SET estado = ?, iniciado_em = '' WHERE estado = ?`, naFila, rodando)
	return err
}

func (f *filaDaIA) avisar() {
	select {
	case f.acorda <- struct{}{}:
	default:
	}
}

func (f *filaDaIA) rodar(ctx context.Context) {
	for {
		g, err := lerGeracao(f.a.db.QueryRowContext(ctx, `SELECT `+colunasGeracao+` FROM geracoes WHERE estado = ? ORDER BY id LIMIT 1`, naFila))
		if err != nil && !errors.Is(err, sql.ErrNoRows) && ctx.Err() == nil {
			log.Printf("fila: %v", err)
		}
		if g == nil {
			select {
			case <-ctx.Done():
				return
			case <-f.acorda:
			}
			continue
		}
		f.executar(ctx, g)
	}
}

func (f *filaDaIA) executar(ctx context.Context, g *geracao) {
	f.a.db.ExecContext(ctx, `UPDATE geracoes SET estado = ?, iniciado_em = ? WHERE id = ?`, rodando, f.a.agora(), g.ID)
	tctx, cancel := context.WithTimeout(ctx, tempoMaximoDaGeracao)
	f.mu.Lock()
	f.atual, f.cancelar = g.ID, cancel
	f.mu.Unlock()

	p, err := f.a.gerar(tctx, g)
	cancelou := errors.Is(tctx.Err(), context.Canceled) && ctx.Err() == nil
	esgotou := errors.Is(tctx.Err(), context.DeadlineExceeded)
	cancel()

	f.mu.Lock()
	f.atual, f.cancelar = 0, nil
	f.mu.Unlock()

	switch {
	case ctx.Err() != nil:
		// O app está parando: a geração volta para a fila no próximo início.
	case cancelou:
		f.a.terminar(g.ID, cancelada, "", nil)
	case esgotou:
		f.a.terminar(g.ID, falhou, "A IA passou de 15 minutos sem terminar.", nil)
	case err != nil:
		log.Printf("geração %d: %v", g.ID, err)
		f.a.terminar(g.ID, falhou, err.Error(), nil)
	default:
		f.a.terminar(g.ID, emRevisao, "", p)
	}
}

func (a *app) terminar(id int64, estado, erro string, p *proposta) {
	texto := ""
	if p != nil {
		b, _ := json.Marshal(p)
		texto = string(b)
	}
	if _, err := a.db.Exec(`UPDATE geracoes SET estado = ?, erro = ?, proposta = CASE WHEN ? = '' THEN proposta ELSE ? END, terminado_em = ? WHERE id = ?`,
		estado, erro, texto, texto, a.agora(), id); err != nil {
		log.Printf("fila: %v", err)
	}
}

// ---------- A geração ----------

var nomesDosTipos = map[string]string{
	"basico":    "frente e verso (um cartão por nota)",
	"invertido": "nos dois sentidos (cada nota vira dois cartões, frente → verso e verso → frente)",
}

func (a *app) gerar(ctx context.Context, g *geracao) (*proposta, error) {
	pasta := filepath.Join(a.cfg.dados, "geracoes", strconv.FormatInt(g.ID, 10))
	if err := os.MkdirAll(pasta, 0o755); err != nil {
		return nil, err
	}
	defer os.RemoveAll(pasta)

	destino := "Os cartões vão para um baralho novo: sugira o nome dele."
	if g.BaralhoID != "" {
		b, err := a.baralho(ctx, g.BaralhoID)
		if err != nil {
			return nil, fmt.Errorf("o baralho de destino não existe mais")
		}
		destino = fmt.Sprintf("Os cartões vão para o baralho que já existe %q (devolva esse mesmo nome em nome_baralho).", b.Nome)
		if frentes := a.frentes(ctx, b.ID); len(frentes) > 0 {
			destino += " Ele já tem estes cartões; não repita nenhum:\n- " + strings.Join(frentes, "\n- ")
		}
	}

	r, err := a.chamarClaude(ctx, pedidoClaude{
		pasta: pasta,
		regra: "gerar-baralho.md",
		instrucao: fmt.Sprintf("O Lucas pediu um baralho de flashcards.\n\nPedido: %q\n\nQuantidade: cerca de %d notas.\n"+
			"Tipo das notas: %s.\n%s\n\nDevolva os dados no formato pedido.",
			g.Pedido, g.Quantidade, nomesDosTipos[g.Tipo], destino),
		esquema: esquemaProposta,
	})
	if err != nil {
		return nil, err
	}
	var p proposta
	if err := dadosDaResposta(r, &p); err != nil {
		return nil, err
	}
	p.NomeBaralho = strings.TrimSpace(p.NomeBaralho)
	cartoes := p.Cartoes[:0]
	for _, c := range p.Cartoes {
		c.Frente, c.Verso, c.Tags, c.Imagem = strings.TrimSpace(c.Frente), strings.TrimSpace(c.Verso), limparTags(c.Tags), strings.TrimSpace(c.Imagem)
		if c.Frente != "" && c.Verso != "" {
			cartoes = append(cartoes, c)
		}
	}
	p.Cartoes = cartoes
	if len(p.Cartoes) == 0 {
		return nil, errors.New("a IA não devolveu nenhum cartão")
	}

	// As fotos vêm da Wikipédia pelo título que a IA indicou; sem foto, o cartão segue sem imagem.
	for i, c := range p.Cartoes {
		if c.Imagem == "" || ctx.Err() != nil {
			continue
		}
		if achadas, err := a.buscarImagens(ctx, c.Imagem, 1); err == nil && len(achadas) > 0 {
			p.Cartoes[i].ImagemURL = achadas[0].URL
		}
	}
	return &p, nil
}

func (a *app) frentes(ctx context.Context, baralhoID string) []string {
	rows, err := a.db.QueryContext(ctx, `SELECT frente FROM notas WHERE baralho_id = ? ORDER BY criada_em LIMIT 500`, baralhoID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var lista []string
	for rows.Next() {
		var f string
		rows.Scan(&f)
		lista = append(lista, strings.ReplaceAll(f, "\n", " "))
	}
	return lista
}

// ---------- Handlers ----------

func (a *app) handlePedirGeracao(w http.ResponseWriter, r *http.Request) {
	var corpo struct {
		Pedido     string `json:"pedido"`
		Quantidade int    `json:"quantidade"`
		Tipo       string `json:"tipo"`
		BaralhoID  string `json:"baralho_id"`
	}
	if !lerCorpo(w, r, &corpo) {
		return
	}
	corpo.Pedido = strings.TrimSpace(corpo.Pedido)
	if corpo.Tipo == "" {
		corpo.Tipo = "basico"
	}
	switch {
	case corpo.Pedido == "":
		erroJSON(w, http.StatusBadRequest, "Diga o que você quer estudar.")
		return
	case corpo.Quantidade < 1 || corpo.Quantidade > 100:
		erroJSON(w, http.StatusBadRequest, "Peça de 1 a 100 cartões.")
		return
	case tiposDeNota[corpo.Tipo] == 0:
		erroJSON(w, http.StatusBadRequest, "Tipo de nota inválido.")
		return
	case corpo.BaralhoID != "" && !a.existeBaralho(r.Context(), corpo.BaralhoID):
		erroJSON(w, http.StatusBadRequest, "Baralho não encontrado.")
		return
	}
	res, err := a.db.ExecContext(r.Context(), `INSERT INTO geracoes (pedido, quantidade, tipo, baralho_id, estado, criado_em) VALUES (?, ?, ?, ?, ?, ?)`,
		corpo.Pedido, corpo.Quantidade, corpo.Tipo, corpo.BaralhoID, naFila, a.agora())
	if err != nil {
		falha(w, err)
		return
	}
	id, _ := res.LastInsertId()
	a.fila.avisar()
	a.responderGeracao(w, r, http.StatusCreated, id)
}

// handleGeracoes lista as que ainda pedem atenção (na fila, rodando, para revisar, com erro) e as
// últimas terminadas.
func (a *app) handleGeracoes(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.QueryContext(r.Context(), `SELECT `+colunasGeracao+` FROM geracoes
		WHERE estado IN (?, ?, ?, ?) OR id IN (SELECT id FROM geracoes ORDER BY id DESC LIMIT 10) ORDER BY id DESC`,
		naFila, rodando, emRevisao, falhou)
	if err != nil {
		falha(w, err)
		return
	}
	defer rows.Close()
	lista := []geracao{}
	for rows.Next() {
		g, err := lerGeracao(rows)
		if err != nil {
			falha(w, err)
			return
		}
		g.Proposta = nil // a lista não precisa dos cartões
		lista = append(lista, *g)
	}
	writeJSON(w, http.StatusOK, lista)
}

func (a *app) handleGeracao(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaGeracao(w, r)
	if ok {
		a.responderGeracao(w, r, http.StatusOK, id)
	}
}

func idDaGeracao(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		erroJSON(w, http.StatusBadRequest, "Id inválido.")
		return 0, false
	}
	return id, true
}

func (a *app) responderGeracao(w http.ResponseWriter, r *http.Request, status int, id int64) {
	g, err := a.geracao(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		erroJSON(w, http.StatusNotFound, "Pedido não encontrado.")
		return
	}
	if err != nil {
		falha(w, err)
		return
	}
	writeJSON(w, status, g)
}

// handleAcaoGeracao: cancelar (na fila ou rodando), refazer (depois de falhar ou ser descartada)
// e descartar (a proposta).
func (a *app) handleAcaoGeracao(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaGeracao(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	var res sql.Result
	var err error
	switch r.PathValue("acao") {
	case "cancelar":
		a.fila.mu.Lock()
		if a.fila.atual == id && a.fila.cancelar != nil {
			a.fila.cancelar()
			a.fila.mu.Unlock()
			a.responderGeracao(w, r, http.StatusOK, id)
			return
		}
		a.fila.mu.Unlock()
		res, err = a.db.ExecContext(ctx, `UPDATE geracoes SET estado = ?, terminado_em = ? WHERE id = ? AND estado = ?`, cancelada, a.agora(), id, naFila)
	case "refazer":
		res, err = a.db.ExecContext(ctx, `UPDATE geracoes SET estado = ?, erro = '', proposta = '', iniciado_em = '', terminado_em = '' WHERE id = ? AND origem = 'ia' AND estado IN (?, ?, ?)`,
			naFila, id, falhou, cancelada, rejeitada)
		a.fila.avisar()
	case "descartar":
		res, err = a.db.ExecContext(ctx, `UPDATE geracoes SET estado = ?, terminado_em = ? WHERE id = ? AND estado = ?`, rejeitada, a.agora(), id, emRevisao)
	default:
		erroJSON(w, http.StatusNotFound, "Ação desconhecida.")
		return
	}
	if err != nil {
		falha(w, err)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		erroJSON(w, http.StatusConflict, "O pedido não está num estado que permita isso.")
		return
	}
	a.responderGeracao(w, r, http.StatusOK, id)
}

// handleAprovar cria as notas que o Lucas manteve, já com as edições dele. As fotos aprovadas são
// baixadas para o servidor só agora; uma que falhar não impede o cartão de entrar.
func (a *app) handleAprovar(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaGeracao(w, r)
	if !ok {
		return
	}
	var corpo struct {
		NomeBaralho string           `json:"nome_baralho"`
		Cartoes     []cartaoProposto `json:"cartoes"`
	}
	if !lerCorpo(w, r, &corpo) {
		return
	}
	ctx := r.Context()
	g, err := a.geracao(ctx, id)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && g.Estado != emRevisao) {
		erroJSON(w, http.StatusConflict, "Esta proposta não está aguardando revisão.")
		return
	}
	if err != nil {
		falha(w, err)
		return
	}
	notas := make([]dadosDaNota, 0, len(corpo.Cartoes))
	for _, c := range corpo.Cartoes {
		d := dadosDaNota{Tipo: g.Tipo, Frente: c.Frente, Verso: c.Verso, Tags: c.Tags}
		if d.validar() != "" || d.Verso == "" {
			erroJSON(w, http.StatusBadRequest, "Todo cartão precisa de frente e verso: “"+cortar(c.Frente, 40)+"”.")
			return
		}
		notas = append(notas, d)
	}
	if len(notas) == 0 {
		erroJSON(w, http.StatusBadRequest, "Nenhum cartão escolhido. Para não aproveitar nada, descarte a proposta.")
		return
	}
	nome := strings.TrimSpace(corpo.NomeBaralho)
	if g.BaralhoID == "" && nome == "" {
		erroJSON(w, http.StatusBadRequest, "Dê um nome ao baralho.")
		return
	}

	semImagem, semAudio := 0, 0
	for i, c := range corpo.Cartoes {
		if c.ImagemURL != "" {
			if notas[i].Imagem, err = a.baixarImagem(ctx, c.ImagemURL); err != nil {
				log.Printf("geração %d: imagem %s: %v", id, c.ImagemURL, err)
				semImagem++
			}
		}
		if c.AudioURL != "" {
			if notas[i].Audio, err = a.baixarAudio(ctx, c.AudioURL); err != nil {
				log.Printf("geração %d: áudio %s: %v", id, c.AudioURL, err)
				semAudio++
			}
		}
	}

	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		falha(w, err)
		return
	}
	defer tx.Rollback()
	baralhoID := g.BaralhoID
	if baralhoID == "" {
		baralhoID = novoID()
		if _, err := tx.ExecContext(ctx, `INSERT INTO baralhos (id, nome, criado_em, atualizado_em) VALUES (?, ?, ?, ?)`, baralhoID, nome, a.agora(), a.agora()); err != nil {
			falha(w, err)
			return
		}
	}
	for _, d := range notas {
		d.BaralhoID = baralhoID
		if _, err := a.criarNota(ctx, tx, d); err != nil {
			falha(w, err)
			return
		}
	}
	res, err := tx.ExecContext(ctx, `UPDATE geracoes SET estado = ?, resultado = ?, terminado_em = ? WHERE id = ? AND estado = ?`,
		concluida, baralhoID, a.agora(), id, emRevisao)
	if err != nil {
		falha(w, err)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		erroJSON(w, http.StatusConflict, "Esta proposta já foi aprovada.")
		return
	}
	if err := tx.Commit(); err != nil {
		falha(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"baralho_id": baralhoID, "criadas": len(notas), "sem_imagem": semImagem, "sem_audio": semAudio})
}

func cortar(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
