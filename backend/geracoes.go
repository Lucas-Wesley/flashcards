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
	ID     int64  `json:"id"`
	Origem string `json:"origem"`
	Pedido string `json:"pedido"`
	// Num texto do Acervo: o texto (só a IA lê), onde ele fica (Tema › capítulo) e o link de volta.
	Texto    string `json:"-"`
	Contexto string `json:"contexto,omitempty"`
	Link     string `json:"link,omitempty"`
	// NomeSugerido: o nome que o Acervo dá ao baralho (o do Tema), quando ele ainda não existe.
	NomeSugerido string    `json:"nome_sugerido,omitempty"`
	Quantidade   int       `json:"quantidade"`
	Tipo         string    `json:"tipo"`
	BaralhoID    string    `json:"baralho_id"`
	Estado       string    `json:"estado"`
	Erro         string    `json:"erro"`
	Proposta     *proposta `json:"proposta,omitempty"`
	Resultado    string    `json:"resultado"`
	CriadoEm     string    `json:"criado_em"`
	IniciadoEm   string    `json:"iniciado_em"`
	TerminadoEm  string    `json:"terminado_em"`
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
	// Imagem: o que a foto deve mostrar (o termo da busca), quando a IA acha que ajuda.
	// ImagemURL: a foto encontrada; só é baixada para o servidor quando o Lucas aprova.
	Imagem    string `json:"imagem"`
	ImagemURL string `json:"imagem_url"`
	// AudioURL: a pronúncia da frente, quando a página de um link traz (baixada ao aprovar).
	// As duas também podem apontar para um arquivo que o app já tem (/api/imagens/…, /api/audios/…):
	// é o caso de um cartão que já existe e vai ser atualizado.
	AudioURL string `json:"audio_url"`
	// Atualiza: o baralho já tem um cartão com esta frente; aprovar muda o verso dele (e o resto)
	// sem perder o histórico de estudo.
	Atualiza bool `json:"atualiza"`
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

const colunasGeracao = `id, origem, pedido, texto, contexto, link, nome_sugerido, quantidade, tipo, baralho_id, estado, erro, proposta, resultado, criado_em, iniciado_em, terminado_em`

func lerGeracao(l linha) (*geracao, error) {
	var g geracao
	var p string
	if err := l.Scan(&g.ID, &g.Origem, &g.Pedido, &g.Texto, &g.Contexto, &g.Link, &g.NomeSugerido, &g.Quantidade, &g.Tipo, &g.BaralhoID, &g.Estado, &g.Erro, &p, &g.Resultado,
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
		if existentes := a.notasDoBaralho(ctx, b.ID); len(existentes) > 0 {
			var lista []string
			for _, n := range existentes {
				lista = append(lista, fmt.Sprintf("%s → %s", linhaUnica(n.Frente), cortar(linhaUnica(n.Verso), 300)))
			}
			destino += " Ele já tem estes cartões (frente → verso). Não crie cartões repetidos. Se o pedido for para mudar ou " +
				"completar cartões que já existem, devolva-os com a frente exatamente igual à de lá: o cartão é atualizado, " +
				"sem perder o histórico de estudo.\n- " + strings.Join(lista, "\n- ")
		}
	}

	pedido := pedidoClaude{
		pasta: pasta,
		regra: "gerar-baralho.md",
		instrucao: fmt.Sprintf("O Lucas pediu um baralho de flashcards.\n\nPedido: %q\n\nQuantidade: cerca de %d notas.\n"+
			"Tipo das notas: %s.\n%s\n\nDevolva os dados no formato pedido.",
			g.Pedido, g.Quantidade, nomesDosTipos[g.Tipo], destino),
		esquema: esquemaProposta,
	}
	if g.Origem == origemAcervo {
		pedido = pedidoDoTexto(g, destino, pasta)
	}
	r, err := a.chamarClaude(ctx, pedido)
	if err != nil {
		return nil, err
	}
	var p proposta
	if err := dadosDaResposta(r, &p); err != nil {
		return nil, err
	}
	p.NomeBaralho = strings.TrimSpace(p.NomeBaralho)
	if g.BaralhoID == "" && g.NomeSugerido != "" {
		p.NomeBaralho = g.NomeSugerido
	}
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

	// Cartão que já existe no baralho: é uma atualização, e a foto e o áudio que ele tem ficam.
	if g.BaralhoID != "" {
		existentes := map[string]nota{}
		for _, n := range a.notasDoBaralho(ctx, g.BaralhoID) {
			existentes[n.Frente] = n
		}
		for i, c := range p.Cartoes {
			if n, ok := existentes[c.Frente]; ok {
				p.Cartoes[i].Atualiza = true
				if n.Imagem != "" {
					p.Cartoes[i].ImagemURL = "/api/imagens/" + n.Imagem
				}
				if n.Audio != "" {
					p.Cartoes[i].AudioURL = "/api/audios/" + n.Audio
				}
			}
		}
	}

	// As fotos vêm da busca pelo que a IA indicou, ordenada pelo cartão inteiro (frente, verso e tags);
	// sem foto, o cartão segue sem imagem.
	for i, c := range p.Cartoes {
		if c.Imagem == "" || c.ImagemURL != "" || ctx.Err() != nil {
			continue
		}
		if achadas, err := a.buscarImagens(ctx, c.Imagem, c.Frente+"\n"+c.Verso+"\n"+c.Tags, 1, false); err == nil && len(achadas) > 0 {
			p.Cartoes[i].ImagemURL = achadas[0].URL
		}
	}
	return &p, nil
}

func (a *app) notasDoBaralho(ctx context.Context, baralhoID string) []nota {
	lista, err := a.notas(ctx, `WHERE baralho_id = ? ORDER BY criada_em LIMIT 500`, baralhoID)
	if err != nil {
		return nil
	}
	return lista
}

func linhaUnica(s string) string {
	return strings.ReplaceAll(strings.TrimSpace(s), "\n", " ")
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
// Com ?baralho=, os pedidos à IA que criaram ou alimentaram aquele baralho.
func (a *app) handleGeracoes(w http.ResponseWriter, r *http.Request) {
	consulta, args := `SELECT `+colunasGeracao+` FROM geracoes
		WHERE estado IN (?, ?, ?, ?) OR id IN (SELECT id FROM geracoes ORDER BY id DESC LIMIT 10) ORDER BY id DESC`,
		[]any{naFila, rodando, emRevisao, falhou}
	if b := r.URL.Query().Get("baralho"); b != "" {
		consulta, args = `SELECT `+colunasGeracao+` FROM geracoes
			WHERE origem IN ('ia', 'acervo') AND (resultado = ? OR baralho_id = ?) ORDER BY id DESC LIMIT 20`, []any{b, b}
	}
	rows, err := a.db.QueryContext(r.Context(), consulta, args...)
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

// handleAcaoGeracao: cancelar (na fila ou rodando), refazer e descartar (a proposta).
//
// Refazer volta o pedido para a fila, com o texto (e a quantidade) editados quando vierem no corpo
// (num texto do Acervo, o pedido é o nome do texto: refaz com o mesmo texto);
// vale depois de terminar de qualquer jeito, inclusive com a proposta ainda em revisão (ela é
// trocada pela nova) ou já aprovada: aí a IA trabalha no baralho que foi criado, e os cartões que
// ela devolver com a mesma frente atualizam os que já existem. Não guarda histórico do pedido.
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
		var corpo struct {
			Pedido     string `json:"pedido"`
			Quantidade int    `json:"quantidade"`
		}
		if r.ContentLength != 0 && !lerCorpo(w, r, &corpo) {
			return
		}
		corpo.Pedido = strings.TrimSpace(corpo.Pedido)
		if corpo.Quantidade != 0 && (corpo.Quantidade < 1 || corpo.Quantidade > 100) {
			erroJSON(w, http.StatusBadRequest, "Peça de 1 a 100 cartões.")
			return
		}
		// Um pedido aprovado num baralho novo passa a mirar esse baralho (se ele ainda existir).
		res, err = a.db.ExecContext(ctx, `UPDATE geracoes SET estado = ?, erro = '', proposta = '', iniciado_em = '', terminado_em = '',
				pedido = CASE WHEN ? = '' THEN pedido ELSE ? END,
				quantidade = CASE WHEN ? = 0 THEN quantidade ELSE ? END,
				baralho_id = CASE WHEN estado = ? AND baralho_id = '' AND resultado IN (SELECT id FROM baralhos) THEN resultado ELSE baralho_id END,
				resultado = ''
			WHERE id = ? AND origem IN ('ia', 'acervo') AND estado IN (?, ?, ?, ?, ?)`,
			naFila, corpo.Pedido, corpo.Pedido, corpo.Quantidade, corpo.Quantidade, concluida,
			id, falhou, cancelada, rejeitada, emRevisao, concluida)
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
		if nome, ok := strings.CutPrefix(c.ImagemURL, "/api/imagens/"); ok && nomeDeImagem.MatchString(nome) {
			notas[i].Imagem = nome
		} else if c.ImagemURL != "" {
			if notas[i].Imagem, err = a.baixarImagem(ctx, c.ImagemURL); err != nil {
				log.Printf("geração %d: imagem %s: %v", id, c.ImagemURL, err)
				semImagem++
			}
		}
		if nome, ok := strings.CutPrefix(c.AudioURL, "/api/audios/"); ok && nomeDeAudio.MatchString(nome) {
			notas[i].Audio = nome
		} else if c.AudioURL != "" {
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
	criadas, atualizadas := 0, 0
	for _, d := range notas {
		d.BaralhoID = baralhoID
		// A mesma frente no baralho: atualiza a nota, e os cartões dela seguem com o histórico.
		var existente, tipo string
		err := tx.QueryRowContext(ctx, `SELECT id, tipo FROM notas WHERE baralho_id = ? AND frente = ? LIMIT 1`, baralhoID, d.Frente).Scan(&existente, &tipo)
		switch {
		case err == nil:
			if tiposDeNota[d.Tipo] < tiposDeNota[tipo] {
				d.Tipo = tipo // atualizar não apaga o cartão do verso
			}
			_, err = tx.ExecContext(ctx, `UPDATE notas SET tipo = ?, verso = ?, tags = ?, imagem = ?, audio = ?, atualizada_em = ? WHERE id = ?`,
				d.Tipo, d.Verso, d.Tags, d.Imagem, d.Audio, a.agora(), existente)
			if err == nil {
				err = criarCartoes(ctx, tx, existente, baralhoID, d.Tipo, a.agora())
			}
			atualizadas++
		case errors.Is(err, sql.ErrNoRows):
			_, err = a.criarNota(ctx, tx, d)
			criadas++
		}
		if err != nil {
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
	writeJSON(w, http.StatusOK, map[string]any{"baralho_id": baralhoID, "criadas": criadas, "atualizadas": atualizadas, "sem_imagem": semImagem, "sem_audio": semAudio})
}

func cortar(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
