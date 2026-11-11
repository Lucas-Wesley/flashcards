package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"golang.org/x/net/html"
)

// Cartões a partir de um link de qualquer site. Primeiro o leitor procura o jeito mais comum de
// mostrar frases de exemplo (a frase em inglês em negrito, uma quebra de linha e a tradução, com o
// áudio logo depois): achando, lê direto, sem IA, e as frases entram exatamente como estão. Se a
// página não segue esse jeito, o texto dela vai para a fila e a IA tira os exemplos
// (regras/extrair-exemplos.md). O resultado vira uma proposta, que o Lucas revisa e aprova.

// fraseDeExemplo: uma frase em inglês, a tradução (com as notas que vierem junto) e o áudio.
type fraseDeExemplo struct {
	Ingles   string
	Traducao string
	Audio    string
}

// ---------- Frases com tradução ----------

// O jeito comum: cada exemplo é um parágrafo com a frase em inglês em negrito, uma quebra de linha
// e a tradução (às vezes com uma nota entre parênteses); o áudio, se houver, vem logo depois:
//
//	<p><em><strong>Sales shot up by 9% last month.</strong><br>As vendas dispararam…</em></p>
//	<figure class="wp-block-audio"><audio src="…/06-sales-shot-up-by-9-last-month.mp3"></audio></figure>
//
// Parágrafos com negrito mas sem quebra de linha (a "resposta rápida", os títulos de cada sentido)
// não são exemplos e ficam de fora.
func lerFrasesComTraducao(pagina *html.Node, endereco *url.URL) (string, []fraseDeExemplo) {
	titulo := ""
	var frases []fraseDeExemplo
	var andar func(n *html.Node)
	andar = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "h1":
				if titulo == "" {
					titulo = textoDe(n)
				}
			case "title":
				if titulo == "" {
					titulo = semNomeDoSite(textoDe(n))
				}
			case "p":
				if f, ok := exemploDoParagrafo(n); ok {
					f.Audio = audioDepois(n, endereco)
					frases = append(frases, f)
					return
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			andar(c)
		}
	}
	andar(pagina)
	// "Shoot Up | O que significa este phrasal verb?" → "Shoot Up"
	if i := strings.Index(titulo, " | "); i > 0 {
		titulo = titulo[:i]
	}
	return strings.TrimSpace(titulo), frases
}

// semNomeDoSite tira o nome do site do fim do <title>: "Shoot Up - Blog de Inglês" → "Shoot Up".
func semNomeDoSite(titulo string) string {
	titulo = limparEspacos(titulo)
	for _, sep := range []string{" - ", " – ", " — "} {
		if i := strings.LastIndex(titulo, sep); i > 0 {
			return titulo[:i]
		}
	}
	return titulo
}

// exemploDoParagrafo separa o parágrafo nas linhas (pelos <br>): a primeira, em negrito, é o
// inglês; as outras, a tradução.
func exemploDoParagrafo(p *html.Node) (fraseDeExemplo, bool) {
	var linhas []string
	var atual strings.Builder
	negritoNaPrimeira := false
	var andar func(n *html.Node, emNegrito bool)
	andar = func(n *html.Node, emNegrito bool) {
		switch {
		case n.Type == html.TextNode:
			if emNegrito && len(linhas) == 0 && strings.TrimSpace(n.Data) != "" {
				negritoNaPrimeira = true
			}
			atual.WriteString(n.Data)
		case n.Type == html.ElementNode && n.Data == "br":
			linhas = append(linhas, atual.String())
			atual.Reset()
		case n.Type == html.ElementNode:
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				andar(c, emNegrito || n.Data == "strong" || n.Data == "b")
			}
		}
	}
	andar(p, false)
	linhas = append(linhas, atual.String())

	var limpas []string
	for _, l := range linhas {
		if l = limparEspacos(l); l != "" {
			limpas = append(limpas, l)
		}
	}
	if !negritoNaPrimeira || len(limpas) < 2 {
		return fraseDeExemplo{}, false
	}
	return fraseDeExemplo{Ingles: limpas[0], Traducao: strings.Join(limpas[1:], "\n")}, true
}

// audioDepois procura o áudio logo depois do parágrafo, pulando parágrafos vazios; para no
// próximo parágrafo com texto, que já é outro exemplo.
func audioDepois(p *html.Node, endereco *url.URL) string {
	for n := proximoElemento(p); n != nil; n = proximoElemento(n) {
		if n.Data == "p" && limparEspacos(textoDe(n)) == "" && n.FirstChild == nil {
			continue
		}
		if n.Data == "figure" || n.Data == "audio" {
			if src := buscarAudio(n); src != "" {
				if u, err := endereco.Parse(src); err == nil {
					return u.String()
				}
			}
		}
		return ""
	}
	return ""
}

func buscarAudio(n *html.Node) string {
	if n.Type == html.ElementNode && (n.Data == "audio" || n.Data == "source") {
		for _, at := range n.Attr {
			if at.Key == "src" && at.Val != "" {
				return at.Val
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if s := buscarAudio(c); s != "" {
			return s
		}
	}
	return ""
}

func proximoElemento(n *html.Node) *html.Node {
	for n = n.NextSibling; n != nil; n = n.NextSibling {
		if n.Type == html.ElementNode {
			return n
		}
	}
	return nil
}

func textoDe(n *html.Node) string {
	var b strings.Builder
	var andar func(*html.Node)
	andar = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			andar(c)
		}
	}
	andar(n)
	return b.String()
}

var espacos = regexp.MustCompile(`\s+`)

func limparEspacos(s string) string {
	return strings.TrimSpace(espacos.ReplaceAllString(strings.ReplaceAll(s, " ", " "), " "))
}

// ---------- O texto da página, para a IA ----------

const tamanhoMaximoDaPagina = 40000

// Partes da página que não são o conteúdo.
var foraDoTexto = map[string]bool{"script": true, "style": true, "noscript": true, "template": true, "svg": true, "iframe": true,
	"nav": true, "header": true, "footer": true, "aside": true, "form": true, "button": true, "select": true}

var blocos = map[string]bool{"p": true, "div": true, "section": true, "article": true, "li": true, "br": true, "h1": true, "h2": true,
	"h3": true, "h4": true, "h5": true, "h6": true, "tr": true, "blockquote": true, "figure": true, "dd": true, "dt": true, "pre": true}

// textoDaPagina devolve o conteúdo da página em texto (o <article> ou o <main>, se houver; senão, o
// <body>), com os negritos marcados com ** e cada áudio marcado com [áudio N] onde aparece, e a
// lista desses áudios.
func textoDaPagina(pagina *html.Node, endereco *url.URL) (string, []string) {
	raiz := buscarElemento(pagina, "article")
	if raiz == nil {
		raiz = buscarElemento(pagina, "main")
	}
	if raiz == nil {
		raiz = buscarElemento(pagina, "body")
	}
	if raiz == nil {
		return "", nil
	}
	var b strings.Builder
	var audios []string
	var andar func(n *html.Node)
	andar = func(n *html.Node) {
		switch n.Type {
		case html.TextNode:
			b.WriteString(n.Data)
			return
		case html.ElementNode:
			if foraDoTexto[n.Data] {
				return
			}
			if n.Data == "audio" {
				if src := buscarAudio(n); src != "" {
					if u, err := endereco.Parse(src); err == nil && (u.Scheme == "https" || u.Scheme == "http") && !slices.Contains(audios, u.String()) {
						audios = append(audios, u.String())
						fmt.Fprintf(&b, " [áudio %d] ", len(audios))
					}
				}
				return
			}
		}
		negrito := n.Type == html.ElementNode && (n.Data == "strong" || n.Data == "b")
		if negrito {
			b.WriteString("**")
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			andar(c)
		}
		if negrito {
			b.WriteString("**")
		}
		if n.Type == html.ElementNode && blocos[n.Data] {
			b.WriteString("\n")
		}
	}
	andar(raiz)
	var linhas []string
	for _, l := range strings.Split(b.String(), "\n") {
		if l = limparEspacos(strings.ReplaceAll(l, "****", "")); l != "" && l != "**" {
			linhas = append(linhas, l)
		}
	}
	texto := strings.Join(linhas, "\n")
	if r := []rune(texto); len(r) > tamanhoMaximoDaPagina {
		texto = string(r[:tamanhoMaximoDaPagina])
	}
	return texto, audios
}

// falaDePhrasalVerb: o título ou o <h1> da página diz que ela explica um phrasal verb ("Shoot Up | O que
// significa este phrasal verb?"). Vale quando a IA não roda; quando roda, quem decide é ela.
func falaDePhrasalVerb(pagina *html.Node) bool {
	for _, nome := range []string{"title", "h1"} {
		if e := buscarElemento(pagina, nome); e != nil && strings.Contains(strings.ToLower(limparEspacos(textoDe(e))), "phrasal verb") {
			return true
		}
	}
	return false
}

func buscarElemento(n *html.Node, nome string) *html.Node {
	if n.Type == html.ElementNode && n.Data == nome {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if e := buscarElemento(c, nome); e != nil {
			return e
		}
	}
	return nil
}

// slug: "Shoot Up" → "shoot-up", para a tag dos cartões do link.
func slug(s string) string {
	var b strings.Builder
	hifen := false
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			hifen = false
		} else if !hifen && b.Len() > 0 {
			b.WriteByte('-')
			hifen = true
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}

// ---------- Do link à proposta ----------

// lerLink baixa a página e tira dela as frases com tradução ou, se ela não as mostra do jeito
// comum, o texto e os áudios, para a IA.
func lerLink(ctx context.Context, endereco string) (paginaDoLink, error) {
	u, err := url.Parse(strings.TrimSpace(endereco))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return paginaDoLink{}, errors.New("Isso não parece um link.")
	}
	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return paginaDoLink{}, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Flashcards-homelab)")
	res, err := clienteWeb.Do(req)
	if err != nil {
		return paginaDoLink{}, fmt.Errorf("A página não abriu: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return paginaDoLink{}, fmt.Errorf("A página respondeu %d.", res.StatusCode)
	}
	if tipo := res.Header.Get("Content-Type"); tipo != "" && !strings.Contains(tipo, "html") {
		return paginaDoLink{}, errors.New("O link não é de uma página.")
	}
	pagina, err := html.Parse(io.LimitReader(res.Body, 5<<20))
	if err != nil {
		return paginaDoLink{}, err
	}
	titulo, frases := lerFrasesComTraducao(pagina, res.Request.URL)
	p := paginaDoLink{Titulo: titulo, Total: len(frases), Frases: frases, PhrasalVerb: falaDePhrasalVerb(pagina)}
	if len(frases) == 0 {
		p.Texto, p.Audios = textoDaPagina(pagina, res.Request.URL)
		if len([]rune(p.Texto)) < 20 {
			return paginaDoLink{}, errors.New("Não achei texto nesta página.")
		}
	}
	if p.Titulo == "" {
		p.Titulo = u.Host
	}
	return p, nil
}

// handleLink lê o link e cria a proposta para revisão: na hora, se a página tem até 3 frases com
// tradução; senão, pela fila, com a IA.
func (a *app) handleLink(w http.ResponseWriter, r *http.Request) {
	var corpo struct {
		URL       string `json:"url"`
		BaralhoID string `json:"baralho_id"`
		Tipo      string `json:"tipo"`
	}
	if !lerCorpo(w, r, &corpo) {
		return
	}
	if corpo.Tipo == "" {
		corpo.Tipo = "basico"
	}
	ctx := r.Context()
	if !tipoDePergunta(corpo.Tipo) {
		erroJSON(w, http.StatusBadRequest, "Tipo de nota inválido.")
		return
	}
	if corpo.BaralhoID != "" && !a.existeBaralho(ctx, corpo.BaralhoID) {
		erroJSON(w, http.StatusBadRequest, "Baralho não encontrado.")
		return
	}
	lida, err := lerLink(ctx, corpo.URL)
	if err != nil {
		erroJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	// Frase que já está no baralho de destino fica de fora (mandar o mesmo link duas vezes não duplica).
	existentes := map[string]bool{}
	if corpo.BaralhoID != "" {
		for _, f := range a.frentes(ctx, corpo.BaralhoID) {
			existentes[f] = true
		}
	}
	pagina := lida
	pagina.Frases = nil
	for _, f := range lida.Frases {
		if existentes[f.Ingles] {
			pagina.Repetidas++
			continue
		}
		existentes[f.Ingles] = true
		pagina.Frases = append(pagina.Frases, f)
	}
	if pagina.Texto == "" && len(pagina.Frases) == 0 {
		erroJSON(w, http.StatusConflict, "Todas as frases deste link já estão no baralho.")
		return
	}

	// Até 3 frases prontas, a proposta sai na hora. Mais que isso, ou sem frases prontas, vai para a IA, na fila.
	estado, texto := emRevisao, ""
	if pagina.Texto != "" || len(pagina.Frases) > exemplosPorPagina {
		estado = naFila
	} else {
		b, _ := json.Marshal(pagina.proposta(pagina.Frases, ""))
		texto = string(b)
	}
	guardada, _ := json.Marshal(pagina)
	res, err := a.db.ExecContext(ctx, `INSERT INTO geracoes (origem, pedido, texto, quantidade, tipo, baralho_id, estado, proposta, criado_em, terminado_em)
		VALUES ('link', ?, ?, ?, ?, ?, ?, ?, ?, ?)`, corpo.URL, string(guardada), exemplosPorPagina, corpo.Tipo, corpo.BaralhoID,
		estado, texto, a.agora(), map[bool]string{true: a.agora(), false: ""}[estado == emRevisao])
	if err != nil {
		falha(w, err)
		return
	}
	id, _ := res.LastInsertId()
	if estado == naFila {
		a.fila.avisar()
	}
	a.responderGeracao(w, r, http.StatusCreated, id)
}

// ---------- Os exemplos mais didáticos ----------

// Uma página costuma ter muitas frases para o mesmo sentido. Viram cartões só as 3 mais
// didáticas. Se a página já tem as frases com tradução, a IA só escolhe (regras/escolher-exemplos.md)
// e devolve os números: as frases, as traduções e os áudios continuam exatamente os da página. Se
// não tem, a IA tira os exemplos do texto (regras/extrair-exemplos.md). As duas são editáveis em Ajustes.
const exemplosPorPagina = 3

// paginaDoLink é o que o leitor tirou da página, guardado no pedido para a IA (e para refazer).
type paginaDoLink struct {
	Titulo    string           `json:"titulo"`
	Total     int              `json:"total"`     // frases com tradução na página
	Repetidas int              `json:"repetidas"` // já estavam no baralho
	Frases    []fraseDeExemplo `json:"frases"`
	// Sem frases com tradução: o texto da página e os áudios dela, na ordem das marcas [áudio N].
	Texto  string   `json:"texto,omitempty"`
	Audios []string `json:"audios,omitempty"`
	// PhrasalVerb: a expressão da página é um phrasal verb; os cartões ganham a tag phrasal-verb.
	PhrasalVerb bool `json:"phrasal_verb,omitempty"`
}

func (p paginaDoLink) proposta(frases []fraseDeExemplo, motivo string) proposta {
	r := proposta{NomeBaralho: p.Titulo}
	tags := slug(p.Titulo)
	if p.PhrasalVerb {
		tags = strings.TrimSpace(tags + " phrasal-verb")
	}
	comAudio := 0
	for _, f := range frases {
		if f.Audio != "" {
			comAudio++
		}
		r.Cartoes = append(r.Cartoes, cartaoProposto{Frente: f.Ingles, Verso: f.Traducao, Tags: tags, AudioURL: f.Audio})
	}
	audio := fmt.Sprintf("%d com áudio", comAudio)
	if comAudio == len(frases) && comAudio > 0 {
		audio = "todas com áudio"
	}
	switch {
	case p.Texto != "":
		r.Observacoes = fmt.Sprintf("%d frases de exemplo tiradas pela IA de “%s”, %s.", len(frases), p.Titulo, audio)
	case len(frases) < len(p.Frases):
		r.Observacoes = fmt.Sprintf("%d das %d frases de exemplo de “%s”, as mais didáticas na escolha da IA, %s.", len(frases), p.Total, p.Titulo, audio)
	default:
		r.Observacoes = fmt.Sprintf("%d frases de exemplo de “%s”, %s.", len(frases), p.Titulo, audio)
	}
	if motivo = strings.TrimSpace(motivo); motivo != "" {
		r.Observacoes += " " + motivo
	}
	if p.Repetidas > 0 {
		r.Observacoes += fmt.Sprintf(" %d já estavam no baralho e ficaram de fora.", p.Repetidas)
	}
	return r
}

const esquemaEscolha = `{"type":"object","properties":{` +
	`"escolhidas":{"type":"array","items":{"type":"integer"}},"phrasal_verb":{"type":"boolean"},"motivo":{"type":"string"}},` +
	`"required":["escolhidas","phrasal_verb","motivo"]}`

const esquemaExtracao = `{"type":"object","properties":{` +
	`"exemplos":{"type":"array","items":{"type":"object","properties":{` +
	`"ingles":{"type":"string"},"traducao":{"type":"string"},"audio":{"type":"integer"}},"required":["ingles","traducao","audio"]}},` +
	`"phrasal_verb":{"type":"boolean"},"motivo":{"type":"string"}},"required":["exemplos","phrasal_verb","motivo"]}`

func (a *app) escolherExemplos(ctx context.Context, g *geracao, pasta string) (*proposta, error) {
	var p paginaDoLink
	if err := json.Unmarshal([]byte(g.Texto), &p); err != nil || (len(p.Frases) == 0 && p.Texto == "") {
		return nil, errors.New("o pedido não tem as frases da página: mande o link de novo")
	}
	if p.Texto != "" {
		return a.extrairExemplos(ctx, g, p, pasta)
	}
	var lista strings.Builder
	for i, f := range p.Frases {
		fmt.Fprintf(&lista, "%d. %s\n   %s\n", i+1, f.Ingles, strings.ReplaceAll(f.Traducao, "\n", " "))
	}
	r, err := a.chamarClaude(ctx, pedidoClaude{
		pasta: pasta,
		regra: "escolher-exemplos.md",
		instrucao: fmt.Sprintf("Página sobre %q (%s). Ela tem estas frases de exemplo, numeradas (inglês e tradução):\n\n%s\n"+
			"Escolha as %d mais didáticas para virarem flashcards. Devolva os números delas em escolhidas; em phrasal_verb, se a expressão da página é um phrasal verb; "+
			"e, em motivo, uma frase curta dizendo o critério.",
			p.Titulo, g.Pedido, lista.String(), exemplosPorPagina),
		esquema:     esquemaEscolha,
		semInternet: true,
	})
	if err != nil {
		return nil, err
	}
	var escolha struct {
		Escolhidas  []int  `json:"escolhidas"`
		PhrasalVerb bool   `json:"phrasal_verb"`
		Motivo      string `json:"motivo"`
	}
	if err := dadosDaResposta(r, &escolha); err != nil {
		return nil, err
	}
	// Na ordem da página, sem repetir, só números que existem, no máximo 3.
	marcadas := map[int]bool{}
	for _, n := range escolha.Escolhidas {
		if n >= 1 && n <= len(p.Frases) && len(marcadas) < exemplosPorPagina {
			marcadas[n] = true
		}
	}
	var frases []fraseDeExemplo
	for i, f := range p.Frases {
		if marcadas[i+1] {
			frases = append(frases, f)
		}
	}
	if len(frases) == 0 {
		return nil, errors.New("a IA não escolheu nenhuma frase da página")
	}
	p.PhrasalVerb = escolha.PhrasalVerb
	pr := p.proposta(frases, escolha.Motivo)
	return &pr, nil
}

// extrairExemplos: a página não mostra as frases do jeito comum, e a IA as tira do texto (que vai
// pela entrada padrão). O áudio de cada uma é o da marca [áudio N] que a IA indicar.
func (a *app) extrairExemplos(ctx context.Context, g *geracao, p paginaDoLink, pasta string) (*proposta, error) {
	existentes := ""
	if g.BaralhoID != "" {
		if f := a.frentes(ctx, g.BaralhoID); len(f) > 0 {
			existentes = "\n\nFrases que já estão no baralho (não repita):\n- " + strings.Join(f, "\n- ")
		}
	}
	audios := "A página não tem áudios."
	if len(p.Audios) > 0 {
		audios = fmt.Sprintf("A página tem %d áudios, marcados no texto como [áudio N] onde aparecem.", len(p.Audios))
	}
	r, err := a.chamarClaude(ctx, pedidoClaude{
		pasta: pasta,
		regra: "extrair-exemplos.md",
		instrucao: fmt.Sprintf("Página %q (%s). O texto dela vem pela entrada; os negritos estão marcados com **. %s%s\n\n"+
			"Tire até %d frases de exemplo em inglês, as mais didáticas, com a tradução. Em audio, o número N do [áudio N] daquela frase, ou 0. "+
			"Em phrasal_verb, se a expressão que a página explica é um phrasal verb. Em motivo, uma frase curta dizendo o critério.",
			p.Titulo, g.Pedido, audios, existentes, exemplosPorPagina),
		entrada:     p.Texto,
		esquema:     esquemaExtracao,
		semInternet: true,
	})
	if err != nil {
		return nil, err
	}
	var extracao struct {
		Exemplos []struct {
			Ingles   string `json:"ingles"`
			Traducao string `json:"traducao"`
			Audio    int    `json:"audio"`
		} `json:"exemplos"`
		PhrasalVerb bool   `json:"phrasal_verb"`
		Motivo      string `json:"motivo"`
	}
	if err := dadosDaResposta(r, &extracao); err != nil {
		return nil, err
	}
	var frases []fraseDeExemplo
	vistas := map[string]bool{}
	for _, e := range extracao.Exemplos {
		f := fraseDeExemplo{Ingles: limparEspacos(strings.ReplaceAll(e.Ingles, "**", "")), Traducao: strings.TrimSpace(strings.ReplaceAll(e.Traducao, "**", ""))}
		if f.Ingles == "" || f.Traducao == "" || vistas[f.Ingles] || len(frases) == exemplosPorPagina {
			continue
		}
		vistas[f.Ingles] = true
		if e.Audio >= 1 && e.Audio <= len(p.Audios) {
			f.Audio = p.Audios[e.Audio-1]
		}
		frases = append(frases, f)
	}
	if len(frases) == 0 {
		return nil, errors.New("a IA não achou frases de exemplo nesta página")
	}
	p.PhrasalVerb = extracao.PhrasalVerb
	pr := p.proposta(frases, extracao.Motivo)
	return &pr, nil
}
