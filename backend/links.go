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

// Cartões a partir de um link. Cada site conhecido tem um leitor próprio (por enquanto, só o do
// Mairo Vergara); o resultado vira uma proposta, como a da IA, que o Lucas revisa e aprova. Não
// passa pela IA: a página já tem as frases prontas, e lê-las direto é instantâneo e exato.

// fraseDeExemplo: uma frase em inglês, a tradução (com as notas que vierem junto) e o áudio.
type fraseDeExemplo struct {
	Ingles   string
	Traducao string
	Audio    string
}

// leitorDeSite extrai o título e as frases de uma página de um site conhecido.
type leitorDeSite func(pagina *html.Node, endereco *url.URL) (titulo string, frases []fraseDeExemplo)

func (a *app) leitorDoLink(u *url.URL) (leitorDeSite, bool) {
	if slices.Contains(a.cfg.sitesMairo, u.Host) {
		return lerMairoVergara, true
	}
	return nil, false
}

// ---------- Mairo Vergara ----------

// No site do Mairo Vergara, cada exemplo é um parágrafo com a frase em inglês em negrito, uma
// quebra de linha e a tradução (às vezes com uma nota entre parênteses); o áudio vem logo depois,
// num <figure class="wp-block-audio">:
//
//	<p><em><strong>Sales shot up by 9% last month.</strong><br>As vendas dispararam…</em></p>
//	<figure class="wp-block-audio"><audio src="…/06-sales-shot-up-by-9-last-month.mp3"></audio></figure>
//
// Parágrafos com negrito mas sem quebra de linha (a "resposta rápida", os títulos de cada sentido)
// não são exemplos e ficam de fora.
func lerMairoVergara(pagina *html.Node, endereco *url.URL) (string, []fraseDeExemplo) {
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
					titulo = strings.TrimSuffix(textoDe(n), " - Mairo Vergara")
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

func (a *app) lerLink(ctx context.Context, endereco string) (string, []fraseDeExemplo, error) {
	u, err := url.Parse(strings.TrimSpace(endereco))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return "", nil, errors.New("Isso não parece um link.")
	}
	leitor, ok := a.leitorDoLink(u)
	if !ok {
		return "", nil, fmt.Errorf("Ainda não sei ler páginas de %s. Por enquanto, só do Mairo Vergara.", u.Host)
	}
	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Flashcards-homelab)")
	res, err := clienteWeb.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("A página não abriu: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("A página respondeu %d.", res.StatusCode)
	}
	pagina, err := html.Parse(io.LimitReader(res.Body, 5<<20))
	if err != nil {
		return "", nil, err
	}
	titulo, frases := leitor(pagina, res.Request.URL)
	if len(frases) == 0 {
		return "", nil, errors.New("Não achei frases com tradução nesta página.")
	}
	return titulo, frases, nil
}

// handleLink lê o link e já cria a proposta para revisão (sem fila: leva um segundo).
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
	if tiposDeNota[corpo.Tipo] == 0 {
		erroJSON(w, http.StatusBadRequest, "Tipo de nota inválido.")
		return
	}
	if corpo.BaralhoID != "" && !a.existeBaralho(ctx, corpo.BaralhoID) {
		erroJSON(w, http.StatusBadRequest, "Baralho não encontrado.")
		return
	}
	titulo, frases, err := a.lerLink(ctx, corpo.URL)
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
	p := proposta{NomeBaralho: titulo}
	comAudio, repetidas := 0, 0
	for _, f := range frases {
		if existentes[f.Ingles] {
			repetidas++
			continue
		}
		existentes[f.Ingles] = true
		if f.Audio != "" {
			comAudio++
		}
		p.Cartoes = append(p.Cartoes, cartaoProposto{Frente: f.Ingles, Verso: f.Traducao, Tags: slug(titulo), AudioURL: f.Audio})
	}
	p.Observacoes = fmt.Sprintf("%d frases de exemplo de “%s”, %s.", len(frases), titulo, map[bool]string{true: "todas com áudio", false: fmt.Sprintf("%d com áudio", comAudio)}[comAudio == len(p.Cartoes) && comAudio > 0])
	if repetidas > 0 {
		p.Observacoes += fmt.Sprintf(" %d já estavam no baralho e ficaram de fora.", repetidas)
	}
	if len(p.Cartoes) == 0 {
		erroJSON(w, http.StatusConflict, "Todas as frases deste link já estão no baralho.")
		return
	}

	texto, _ := json.Marshal(p)
	res, err := a.db.ExecContext(ctx, `INSERT INTO geracoes (origem, pedido, quantidade, tipo, baralho_id, estado, proposta, criado_em, terminado_em)
		VALUES ('link', ?, ?, ?, ?, ?, ?, ?, ?)`, corpo.URL, len(p.Cartoes), corpo.Tipo, corpo.BaralhoID, emRevisao, string(texto), a.agora(), a.agora())
	if err != nil {
		falha(w, err)
		return
	}
	id, _ := res.LastInsertId()
	a.responderGeracao(w, r, http.StatusCreated, id)
}
