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

// A IA pesquisa para conferir os fatos, então uma geração pode levar alguns minutos; um conjunto
// grande (os países do mundo, com duas imagens cada) leva mais.
const tempoMaximoDaGeracao = 30 * time.Minute

// Quantidade: opcional no pedido à IA (0 = ela decide pelo pedido, F39); o teto é o de um conjunto
// grande, como os países do mundo.
const maximoDeCartoes = 300

// maximoDaInstrucao (F42): a instrução extra de um link ou de um texto é curta, não um pedido novo.
const maximoDaInstrucao = 1000

type geracao struct {
	ID     int64  `json:"id"`
	Origem string `json:"origem"`
	Pedido string `json:"pedido"`
	// Instrucao (F42): o que o Lucas acrescentou ao refazer um link ou um texto do Acervo ("quero 5
	// frases", "foque nas datas"), que não tem um pedido escrito para editar.
	Instrucao string `json:"instrucao,omitempty"`
	// Num texto do Acervo: o texto (só a IA lê), onde ele fica (Tema › capítulo) e o link de volta.
	Texto    string `json:"-"`
	Contexto string `json:"contexto,omitempty"`
	Link     string `json:"link,omitempty"`
	// NomeSugerido: o nome do baralho novo, que o Lucas deu no Acervo.
	NomeSugerido string `json:"nome_sugerido,omitempty"`
	// Secoes: os capítulos do texto, cada um com o link dele no Acervo (JSON de []secaoDoTexto).
	Secoes      string    `json:"-"`
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
	// PodeRefazer: o pedido passou (ou passa) pela IA; um link com até 3 frases prontas não.
	PodeRefazer bool `json:"pode_refazer"`
}

func (g *geracao) podeRefazer() bool {
	if g.Origem != "link" {
		return true
	}
	var p paginaDoLink
	return json.Unmarshal([]byte(g.Texto), &p) == nil && (p.Texto != "" || len(p.Frases) > g.exemplos())
}

type proposta struct {
	NomeBaralho string `json:"nome_baralho"`
	// Entendimento (F39): como a IA entendeu o pedido (o formato do cartão e o que ele cobre), para o
	// Lucas ver logo se ela captou a ideia.
	Entendimento string           `json:"entendimento,omitempty"`
	Observacoes  string           `json:"observacoes"`
	Cartoes      []cartaoProposto `json:"cartoes"`
	// Conferencia (F40): o que a segunda leitura achou da proposta como um todo.
	Conferencia *conferencia `json:"conferencia,omitempty"`
}

type cartaoProposto struct {
	Frente string `json:"frente"`
	Verso  string `json:"verso"`
	Tags   string `json:"tags"`
	// Imagem: o que a foto deve mostrar (o termo da busca), quando a IA acha que ajuda.
	// ImagemURL: a foto encontrada; só é baixada para o servidor quando o Lucas aprova.
	Imagem    string `json:"imagem"`
	ImagemURL string `json:"imagem_url"`
	// ImagemFrente (F39): a imagem que aparece com a pergunta (a bandeira a reconhecer), do mesmo jeito.
	ImagemFrente    string `json:"imagem_frente,omitempty"`
	ImagemFrenteURL string `json:"imagem_frente_url,omitempty"`
	// AudioURL: a pronúncia da frente, quando a página de um link traz (baixada ao aprovar).
	// As duas também podem apontar para um arquivo que o app já tem (/api/imagens/…, /api/audios/…):
	// é o caso de um cartão que já existe e vai ser atualizado.
	AudioURL string `json:"audio_url"`
	// Atualiza: o baralho já tem um cartão com esta frente; aprovar muda o verso dele (e o resto)
	// sem perder o histórico de estudo.
	Atualiza bool `json:"atualiza"`
	// Num texto do Acervo: o capítulo de onde o cartão saiu (a IA escolhe entre os do texto) e o
	// link para ele, que a nota guarda para abrir no estudo.
	Secao string `json:"secao,omitempty"`
	Fonte string `json:"fonte,omitempty"`
	// Avisos (F40): o que pede um olhar do Lucas neste cartão (uma imagem que não veio ou veio da
	// busca, o que a conferência apontou).
	Avisos []string `json:"avisos,omitempty"`
	// O arquivo de cada imagem encontrada, para a conferência ver o que veio (não fica guardado).
	tituloImagem, tituloImagemFrente string
}

const esquemaProposta = `{
  "type": "object",
  "properties": {
    "nome_baralho": {"type": "string"},
    "entendimento": {"type": "string"},
    "observacoes": {"type": "string"},
    "cartoes": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "frente": {"type": "string"},
          "verso": {"type": "string"},
          "tags": {"type": "string"},
          "imagem": {"type": "string"},
          "imagem_frente": {"type": "string"},
          "secao": {"type": "string"}
        },
        "required": ["frente", "verso"]
      }
    }
  },
  "required": ["nome_baralho", "cartoes"]
}`

const colunasGeracao = `id, origem, pedido, instrucao, texto, contexto, link, nome_sugerido, secoes, quantidade, tipo, baralho_id, estado, erro, proposta, resultado, criado_em, iniciado_em, terminado_em`

func lerGeracao(l linha) (*geracao, error) {
	var g geracao
	var p string
	if err := l.Scan(&g.ID, &g.Origem, &g.Pedido, &g.Instrucao, &g.Texto, &g.Contexto, &g.Link, &g.NomeSugerido, &g.Secoes, &g.Quantidade, &g.Tipo, &g.BaralhoID, &g.Estado, &g.Erro, &p, &g.Resultado,
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
		msg := fmt.Sprintf("A IA passou de %d minutos sem terminar.", int(tempoMaximoDaGeracao.Minutes()))
		f.a.terminar(g.ID, falhou, msg, nil)
		f.a.avisarGeracao(g, msg)
	case err != nil:
		log.Printf("geração %d: %v", g.ID, err)
		f.a.terminar(g.ID, falhou, err.Error(), nil)
		f.a.avisarGeracao(g, err.Error())
	default:
		f.a.terminar(g.ID, emRevisao, "", p)
		f.a.avisarGeracao(g, "")
	}
}

// avisarGeracao (F35): a proposta da IA espera revisão, ou a geração falhou (com o erro).
func (a *app) avisarGeracao(g *geracao, erro string) {
	av := aviso{
		Titulo: primeiro(g.NomeSugerido, g.Contexto, g.Pedido, g.Link, "Cartões"),
		Texto:  "Cartões prontos para revisar.",
		Link:   fmt.Sprintf("%s/#/geracao/%d", a.cfg.link, g.ID),
	}
	if erro != "" {
		av.Texto, av.Urgente = "A geração falhou: "+erro, true
	}
	a.avisos.avisar(av)
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
	"leitura":   "de leitura (a frente é uma pergunta aberta; o verso, um texto curto para ler)",
}

// textoDaQuantidade: sem número, a IA decide pelo pedido; com número, ele é uma referência, e um
// conjunto completo pedido com todas as letras ("todos os estados") vale mais que ele (F39).
func textoDaQuantidade(n int) string {
	if n <= 0 {
		return "Quantidade: o Lucas não disse; decida pelo pedido (ver \"Quantos cartões\" nas instruções)."
	}
	return fmt.Sprintf("Quantidade: o Lucas sugeriu cerca de %d notas. É uma referência, não um teto: se o pedido nomeia um "+
		"conjunto completo (todos os estados, todos os países), cubra o conjunto inteiro e diga em observacoes que passou do número.", n)
}

// textoDaInstrucao (F42): a instrução extra do Lucas, quando houver, no fim do pedido à IA.
func (g *geracao) textoDaInstrucao() string {
	if strings.TrimSpace(g.Instrucao) == "" {
		return ""
	}
	return fmt.Sprintf("\n\nInstrução extra do Lucas para este pedido (vale mais que os critérios padrão das instruções, "+
		"sem quebrar as regras de fonte delas): %q", strings.TrimSpace(g.Instrucao))
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
				com := ""
				if n.ImagemFrente != "" {
					com = " [com imagem na frente]"
				}
				lista = append(lista, fmt.Sprintf("%s → %s%s", linhaUnica(n.Frente), cortar(linhaUnica(n.Verso), 300), com))
			}
			destino += " Ele já tem estes cartões (frente → verso). Não crie cartões repetidos. Se o pedido for para mudar ou " +
				"completar cartões que já existem, devolva-os com a frente exatamente igual à de lá (e, num cartão com imagem " +
				"na frente, também o verso igual): o cartão é atualizado, sem perder o histórico de estudo. Um pedido de um " +
				"assunto novo no mesmo baralho não muda os cartões que já existem.\n- " + strings.Join(lista, "\n- ")
		}
	}

	pedido := pedidoClaude{
		pasta: pasta,
		regra: "gerar-baralho.md",
		instrucao: fmt.Sprintf("O Lucas pediu um baralho de flashcards.\n\nPedido: %q\n\n%s\n"+
			"Tipo das notas: %s.\n%s\n\nDevolva os dados no formato pedido.",
			g.Pedido, textoDaQuantidade(g.Quantidade), nomesDosTipos[g.Tipo], destino),
		esquema: esquemaProposta,
	}
	if g.doTexto() {
		pedido = pedidoDoTexto(g, destino, pasta)
	}
	pedido.instrucao += g.textoDaInstrucao()
	if g.Origem == "link" {
		return a.escolherExemplos(ctx, g, pasta)
	}
	r, err := a.chamarClaude(ctx, pedido)
	if err != nil {
		return nil, err
	}
	var p proposta
	if err := dadosDaResposta(r, &p); err != nil {
		return nil, err
	}
	p.NomeBaralho, p.Entendimento = strings.TrimSpace(p.NomeBaralho), strings.TrimSpace(p.Entendimento)
	if g.BaralhoID == "" && g.NomeSugerido != "" {
		p.NomeBaralho = g.NomeSugerido
	}
	cartoes := p.Cartoes[:0]
	for _, c := range p.Cartoes {
		c.Frente, c.Verso, c.Tags, c.Imagem = strings.TrimSpace(c.Frente), strings.TrimSpace(c.Verso), limparTags(c.Tags), strings.TrimSpace(c.Imagem)
		c.ImagemFrente = strings.TrimSpace(c.ImagemFrente)
		if g.doTexto() {
			c.Secao, c.Fonte = g.fonteDoCartao(c.Secao)
		} else {
			c.Secao = ""
		}
		if c.Frente != "" && c.Verso != "" {
			cartoes = append(cartoes, c)
		}
	}
	p.Cartoes = cartoes
	if len(p.Cartoes) == 0 {
		return nil, errors.New("a IA não devolveu nenhum cartão")
	}

	// Cartão que já existe no baralho: é uma atualização, e as fotos e o áudio que ele tem ficam.
	if g.BaralhoID != "" {
		existentes := map[string]nota{}
		for _, n := range a.notasDoBaralho(ctx, g.BaralhoID) {
			existentes[chaveDaNota(n.Frente, n.Verso, n.ImagemFrente != "")] = n
		}
		for i, c := range p.Cartoes {
			if n, ok := existentes[chaveDaNota(c.Frente, c.Verso, c.ImagemFrente != "")]; ok {
				p.Cartoes[i].Atualiza = true
				if n.Imagem != "" {
					p.Cartoes[i].ImagemURL = "/api/imagens/" + n.Imagem
				}
				if n.ImagemFrente != "" {
					p.Cartoes[i].ImagemFrenteURL = "/api/imagens/" + n.ImagemFrente
				}
				if n.Audio != "" {
					p.Cartoes[i].AudioURL = "/api/audios/" + n.Audio
				}
			}
		}
	}

	a.buscarFotosDaProposta(ctx, p.Cartoes)
	// Os cartões de um texto do Acervo ou de uma marcação saem só do texto, que a conferência não lê.
	if !g.doTexto() {
		a.conferir(ctx, g, &p, pasta)
	}
	return &p, nil
}

// buscasAoMesmoTempo: a busca por termo consulta duas ou três fontes de uma vez; mais que poucas
// ao mesmo tempo, a Wikimedia passa a recusar (429), como no baralho dos países (F39).
const buscasAoMesmoTempo = 2

// buscarFotosDaProposta: as fotos vêm do que a IA indicou. Os arquivos exatos do Commons
// ("Arquivo:Flag of Japan.svg") vêm todos juntos, em lotes; o resto (e o arquivo que não existe)
// vem da busca pelo termo, ordenada pelo cartão inteiro (frente, verso e tags). Sem foto, o
// cartão segue sem imagem. A foto que não veio, e a que veio da busca no lugar de um arquivo
// exato (no baralho dos países, a lua no lugar do mapa de Liechtenstein), ficam como aviso no
// cartão (F40).
func (a *app) buscarFotosDaProposta(ctx context.Context, cartoes []cartaoProposto) {
	type pedidoDeFoto struct {
		cartao          int
		lado            string
		termo, contexto string
		destino, titulo *string
		buscou          bool // veio da busca pelo termo, não de um arquivo pedido
	}
	var pedidos []*pedidoDeFoto
	var nomes []string
	vistos := map[string]bool{}
	for i := range cartoes {
		c := &cartoes[i]
		contexto := c.Frente + "\n" + c.Verso + "\n" + c.Tags
		for _, p := range []*pedidoDeFoto{
			{cartao: i, lado: "do verso", termo: c.Imagem, contexto: contexto, destino: &c.ImagemURL, titulo: &c.tituloImagem},
			{cartao: i, lado: "da frente", termo: c.ImagemFrente, contexto: contexto, destino: &c.ImagemFrenteURL, titulo: &c.tituloImagemFrente},
		} {
			if p.termo == "" || *p.destino != "" {
				continue
			}
			pedidos = append(pedidos, p)
			for _, nome := range arquivosDoTermo(alternativasDoTermo(p.termo)) {
				if !vistos[nome] {
					vistos[nome] = true
					nomes = append(nomes, nome)
				}
			}
		}
	}
	exatos, _ := a.arquivosDoCommons(ctx, nomes)

	vagas := make(chan struct{}, buscasAoMesmoTempo)
	var wg sync.WaitGroup
	for _, p := range pedidos {
		alternativas := alternativasDoTermo(p.termo)
		if im, achou := primeiroAchado(arquivosDoTermo(alternativas), exatos); achou {
			*p.destino, *p.titulo = im.URL, im.Titulo
			continue
		}
		// Nenhum arquivo existe: busca pelo primeiro termo, sem conferir os arquivos de novo.
		termo := termoDaBusca(alternativas)
		if termo == "" {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			vagas <- struct{}{}
			defer func() { <-vagas }()
			if ctx.Err() != nil {
				return
			}
			if achadas, err := a.buscarNasFontes(ctx, termo, p.contexto, 1, false); err == nil && len(achadas) > 0 {
				*p.destino, *p.titulo, p.buscou = achadas[0].URL, achadas[0].Titulo, true
			}
		}()
	}
	wg.Wait()

	for _, p := range pedidos {
		c := &cartoes[p.cartao]
		pediuArquivo := len(arquivosDoTermo(alternativasDoTermo(p.termo))) > 0
		switch {
		case *p.destino == "":
			c.Avisos = append(c.Avisos, fmt.Sprintf("A imagem %s não veio (a IA pediu: %s).", p.lado, p.termo))
		case p.buscou && pediuArquivo:
			c.Avisos = append(c.Avisos, fmt.Sprintf("O arquivo pedido para a imagem %s não existe; veio da busca (%s). Confira.", p.lado, *p.titulo))
		}
	}
}

// chaveDaNota: a frente identifica a nota no baralho; com imagem na frente, a frente pode ser a
// mesma em todas ("De que país é esta bandeira?"), e o verso entra junto (F39).
func chaveDaNota(frente, verso string, imagemNaFrente bool) string {
	if imagemNaFrente {
		return frente + "\x00" + verso
	}
	return frente
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
	case corpo.Quantidade < 0 || corpo.Quantidade > maximoDeCartoes:
		erroJSON(w, http.StatusBadRequest, fmt.Sprintf("A quantidade vai de 1 a %d (ou fica em branco, e a IA decide).", maximoDeCartoes))
		return
	case !tipoDePergunta(corpo.Tipo):
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
	g.PodeRefazer = g.podeRefazer()
	writeJSON(w, status, g)
}

// handleExcluirGeracao apaga o pedido, em qualquer estado (rodando, a IA é parada antes). Os
// cartões de um pedido já aprovado continuam no baralho.
func (a *app) handleExcluirGeracao(w http.ResponseWriter, r *http.Request) {
	id, ok := idDaGeracao(w, r)
	if !ok {
		return
	}
	a.fila.mu.Lock()
	if a.fila.atual == id && a.fila.cancelar != nil {
		a.fila.cancelar()
	}
	a.fila.mu.Unlock()
	res, err := a.db.ExecContext(r.Context(), `DELETE FROM geracoes WHERE id = ?`, id)
	if err != nil {
		falha(w, err)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		erroJSON(w, http.StatusNotFound, "Pedido não encontrado.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
		// Quantidade: ausente, fica a do pedido; 0, a IA decide (F39).
		// Instrucao (F42): presente, troca a instrução extra (vazia, apaga).
		var corpo struct {
			Pedido     string  `json:"pedido"`
			Quantidade *int    `json:"quantidade"`
			Instrucao  *string `json:"instrucao"`
		}
		if r.ContentLength != 0 && !lerCorpo(w, r, &corpo) {
			return
		}
		corpo.Pedido = strings.TrimSpace(corpo.Pedido)
		quantidade := -1
		if corpo.Quantidade != nil {
			quantidade = *corpo.Quantidade
			if quantidade < 0 || quantidade > maximoDeCartoes {
				erroJSON(w, http.StatusBadRequest, fmt.Sprintf("A quantidade vai de 1 a %d (ou fica em branco, e a IA decide).", maximoDeCartoes))
				return
			}
		}
		instrucao := ""
		if corpo.Instrucao != nil {
			instrucao = strings.TrimSpace(*corpo.Instrucao)
			if len([]rune(instrucao)) > maximoDaInstrucao {
				erroJSON(w, http.StatusBadRequest, fmt.Sprintf("A instrução vai até %d caracteres.", maximoDaInstrucao))
				return
			}
		}
		g, gerr := a.geracao(ctx, id)
		// Num texto do Acervo, a quantidade é um teto: 0 não quer dizer nada, e fica a do pedido.
		if gerr == nil && g.doTexto() && quantidade == 0 {
			quantidade = -1
		}
		if gerr == nil && g.Origem == "link" && quantidade > maximoDeExemplos {
			erroJSON(w, http.StatusBadRequest, fmt.Sprintf("Um link dá até %d frases.", maximoDeExemplos))
			return
		}
		// Um link só é refeito quando passa pela IA: a página tinha mais frases que as pedidas, não as mostrava
		// prontas, ou agora vai com uma instrução ou outra quantidade (F42).
		if gerr == nil && !g.podeRefazer() && instrucao == "" && quantidade < 0 {
			erroJSON(w, http.StatusConflict, "Este link não passou pela IA: não há o que refazer.")
			return
		}
		// Um pedido aprovado num baralho novo passa a mirar esse baralho (se ele ainda existir).
		res, err = a.db.ExecContext(ctx, `UPDATE geracoes SET estado = ?, erro = '', proposta = '', iniciado_em = '', terminado_em = '',
				pedido = CASE WHEN ? = '' THEN pedido ELSE ? END,
				quantidade = CASE WHEN ? < 0 THEN quantidade ELSE ? END,
				instrucao = CASE WHEN ? THEN ? ELSE instrucao END,
				baralho_id = CASE WHEN estado = ? AND baralho_id = '' AND resultado IN (SELECT id FROM baralhos) THEN resultado ELSE baralho_id END,
				resultado = ''
			WHERE id = ? AND origem IN ('ia', 'acervo', 'biblioteca', 'link') AND estado IN (?, ?, ?, ?, ?)`,
			naFila, corpo.Pedido, corpo.Pedido, quantidade, quantidade, corpo.Instrucao != nil, instrucao, concluida,
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
		// BaralhoID: o baralho escolhido na revisão ("" = um novo, com NomeBaralho); sem ele, o do pedido.
		BaralhoID   *string          `json:"baralho_id"`
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
		d := dadosDaNota{Tipo: g.Tipo, Frente: c.Frente, Verso: c.Verso, Tags: c.Tags, Fonte: c.Fonte}
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
	baralhoID := g.BaralhoID
	if corpo.BaralhoID != nil {
		baralhoID = *corpo.BaralhoID
		if baralhoID != "" && !a.existeBaralho(ctx, baralhoID) {
			erroJSON(w, http.StatusBadRequest, "Baralho não encontrado.")
			return
		}
	}
	nome := strings.TrimSpace(corpo.NomeBaralho)
	if baralhoID == "" && nome == "" {
		erroJSON(w, http.StatusBadRequest, "Dê um nome ao baralho.")
		return
	}
	// Áudio só dos endereços que vieram na proposta (os que o leitor achou na página do link).
	audios := map[string]bool{}
	if g.Proposta != nil {
		for _, c := range g.Proposta.Cartoes {
			audios[c.AudioURL] = true
		}
	}

	semImagem, semAudio := 0, 0
	trazerImagem := func(endereco string) string {
		if nome, ok := strings.CutPrefix(endereco, "/api/imagens/"); ok && nomeDeImagem.MatchString(nome) {
			return nome
		}
		if endereco == "" {
			return ""
		}
		nome, err := a.baixarImagem(ctx, endereco)
		if err != nil {
			log.Printf("geração %d: imagem %s: %v", id, endereco, err)
			semImagem++
		}
		return nome
	}
	for i, c := range corpo.Cartoes {
		notas[i].Imagem = trazerImagem(c.ImagemURL)
		notas[i].ImagemFrente = trazerImagem(c.ImagemFrenteURL)
		if nome, ok := strings.CutPrefix(c.AudioURL, "/api/audios/"); ok && nomeDeAudio.MatchString(nome) {
			notas[i].Audio = nome
		} else if c.AudioURL != "" {
			if !audios[c.AudioURL] {
				err = errors.New("não é um áudio da página")
			} else {
				notas[i].Audio, err = a.baixarAudio(ctx, c.AudioURL)
			}
			if err != nil {
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
	if baralhoID == "" {
		baralhoID = novoID()
		if _, err := tx.ExecContext(ctx, `INSERT INTO baralhos (id, nome, criado_em, atualizado_em) VALUES (?, ?, ?, ?)`, baralhoID, nome, a.agora(), a.agora()); err != nil {
			falha(w, err)
			return
		}
	}
	criadas, atualizadas := 0, 0
	for i, d := range notas {
		d.BaralhoID = baralhoID
		// A mesma frente no baralho: atualiza a nota, e os cartões dela seguem com o histórico. Com
		// imagem na frente, o verso entra junto, mesmo que a imagem não tenha vindo agora.
		var existente, tipo string
		comImagem := corpo.Cartoes[i].ImagemFrenteURL != ""
		err := tx.QueryRowContext(ctx, `SELECT id, tipo FROM notas WHERE baralho_id = ? AND frente = ? AND (? = 0 OR (verso = ? AND imagem_frente != '')) LIMIT 1`,
			baralhoID, d.Frente, comImagem, d.Verso).Scan(&existente, &tipo)
		switch {
		case err == nil:
			if tipoDePergunta(d.Tipo) && tiposDeNota[d.Tipo] < tiposDeNota[tipo] {
				d.Tipo = tipo // atualizar não apaga o cartão do verso
			}
			_, err = tx.ExecContext(ctx, `UPDATE notas SET tipo = ?, verso = ?, tags = ?, imagem = ?, imagem_frente = ?, audio = ?,
					fonte = CASE WHEN ? = '' THEN fonte ELSE ? END, atualizada_em = ? WHERE id = ?`,
				d.Tipo, d.Verso, d.Tags, d.Imagem, d.ImagemFrente, d.Audio, d.Fonte, d.Fonte, a.agora(), existente)
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
	res, err := tx.ExecContext(ctx, `UPDATE geracoes SET estado = ?, resultado = ?, baralho_id = ?, terminado_em = ? WHERE id = ? AND estado = ?`,
		concluida, baralhoID, baralhoID, a.agora(), id, emRevisao)
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

// primeiro devolve o primeiro valor não vazio.
func primeiro(valores ...string) string {
	for _, v := range valores {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func cortar(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
