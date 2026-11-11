package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"
)

// Flashcards de um texto do Acervo (ou de uma marcação da Biblioteca, F36): lá, o Lucas aperta "Gerar flashcards" num Tema, num capítulo ou
// numa Referência, e o Acervo manda o texto para cá. Vira um pedido à IA como os outros (fila,
// proposta, revisão), só que os cartões saem do texto, sem a IA ir à internet. O Lucas escolhe lá o
// baralho: um que já existe (os cartões de mesma frente são atualizados) ou um novo, com o nome
// que ele deu. Cada cartão guarda o link do capítulo de onde saiu, para abrir no Acervo.

const origemAcervo = "acervo"

// Marcação da Biblioteca (F36): o texto é o trecho marcado (curto), e o contexto traz o livro, o
// capítulo, a nota do Lucas e a página em volta. A IA tira poucos cartões, só do trecho.
const (
	origemBiblioteca    = "biblioteca"
	tamanhoMaximoTrecho = 20_000
	cartoesDaMarcacao   = 3
)

// doTexto: o pedido traz um texto pronto (do Acervo ou de uma marcação), em vez de um pedido livre.
func (g *geracao) doTexto() bool {
	return g.Origem == origemAcervo || g.Origem == origemBiblioteca
}

// Um Tema inteiro pode ser grande; acima disso o Acervo pede para mandar por capítulo.
const tamanhoMaximoDoTexto = 150_000

// secaoDoTexto: um capítulo do texto ("## Título") e o link dele no Acervo.
type secaoDoTexto struct {
	Titulo string `json:"titulo"`
	Link   string `json:"link"`
}

func (a *app) handlePedirDoTexto(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4*tamanhoMaximoDoTexto)
	var corpo struct {
		Titulo    string         `json:"titulo"`     // "Tema › Capítulo"
		BaralhoID string         `json:"baralho_id"` // um baralho que já existe; vazio = baralho novo
		Baralho   string         `json:"baralho"`    // o nome do baralho novo
		Contexto  string         `json:"contexto"`   // de onde vem o texto, para a IA entender
		Texto     string         `json:"texto"`
		Link      string         `json:"link"` // o texto no Acervo
		Secoes    []secaoDoTexto `json:"secoes"`
		Tipo      string         `json:"tipo"`
		Origem    string         `json:"origem"` // "acervo" (o padrão) ou "biblioteca"
	}
	if !lerCorpo(w, r, &corpo) {
		return
	}
	corpo.Titulo, corpo.Baralho, corpo.Texto = strings.TrimSpace(corpo.Titulo), strings.TrimSpace(corpo.Baralho), strings.TrimSpace(corpo.Texto)
	if corpo.Tipo == "" {
		corpo.Tipo = "basico"
	}
	if corpo.Origem == "" {
		corpo.Origem = origemAcervo
	}
	tamanho := utf8.RuneCountInString(corpo.Texto)
	marcacao := corpo.Origem == origemBiblioteca
	quantidade := limiteDeCartoes(corpo.Texto)
	if marcacao {
		quantidade = cartoesDaMarcacao
	}
	if corpo.Tipo == "leitura" {
		quantidade = cartoesDeLeitura
	}
	switch {
	case corpo.Origem != origemAcervo && !marcacao:
		erroJSON(w, http.StatusBadRequest, "Origem inválida.")
		return
	case corpo.Titulo == "":
		erroJSON(w, http.StatusBadRequest, "Diga de onde vem o texto.")
		return
	case marcacao && tamanho < 2:
		erroJSON(w, http.StatusBadRequest, "A marcação está vazia.")
		return
	case marcacao && tamanho > tamanhoMaximoTrecho:
		erroJSON(w, http.StatusRequestEntityTooLarge, "A marcação é grande demais para um cartão. Marque um trecho menor.")
		return
	case !marcacao && tamanho < 200:
		erroJSON(w, http.StatusBadRequest, "O texto é curto demais para virar cartões.")
		return
	case tamanho > tamanhoMaximoDoTexto:
		erroJSON(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("O texto tem %d mil caracteres; o máximo é %d mil. Gere por capítulo.", tamanho/1000, tamanhoMaximoDoTexto/1000))
		return
	case tiposDeNota[corpo.Tipo] == 0 || (marcacao && !tipoDePergunta(corpo.Tipo)):
		erroJSON(w, http.StatusBadRequest, "Tipo de nota inválido.")
		return
	case corpo.BaralhoID != "" && !a.existeBaralho(r.Context(), corpo.BaralhoID):
		erroJSON(w, http.StatusBadRequest, "Esse baralho não existe mais. Escolha outro.")
		return
	case corpo.BaralhoID == "" && corpo.Baralho == "":
		erroJSON(w, http.StatusBadRequest, "Escolha um baralho ou dê o nome do baralho novo.")
		return
	}
	if corpo.BaralhoID != "" {
		corpo.Baralho = ""
	}
	secoes := []secaoDoTexto{}
	for _, s := range corpo.Secoes {
		s.Titulo, s.Link = strings.TrimSpace(s.Titulo), strings.TrimSpace(s.Link)
		if s.Titulo != "" && linkDeFonte(s.Link) {
			secoes = append(secoes, s)
		}
	}
	jsonSecoes, _ := json.Marshal(secoes)
	link := strings.TrimSpace(corpo.Link)
	if !linkDeFonte(link) {
		link = ""
	}

	res, err := a.db.ExecContext(r.Context(), `INSERT INTO geracoes (origem, pedido, texto, contexto, link, nome_sugerido, secoes, quantidade, tipo, baralho_id, estado, criado_em)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		corpo.Origem, corpo.Titulo, corpo.Texto, strings.TrimSpace(corpo.Contexto), link, corpo.Baralho, string(jsonSecoes),
		quantidade, corpo.Tipo, corpo.BaralhoID, naFila, a.agora())
	if err != nil {
		falha(w, err)
		return
	}
	id, _ := res.LastInsertId()
	a.fila.avisar()
	a.responderGeracao(w, r, http.StatusCreated, id)
}

// limiteDeCartoes: o teto que a IA recebe. Ela decide quantos o texto rende; um texto longo não
// deve virar uma pilha rasa de cartões, nem um curto ser esticado.
// cartoesDeLeitura (F38): um texto vira um cartão de leitura; mais só quando muda de assunto.
const cartoesDeLeitura = 4

func limiteDeCartoes(texto string) int {
	return min(100, max(5, len(strings.Fields(texto))/80))
}

func (g *geracao) secoes() []secaoDoTexto {
	var lista []secaoDoTexto
	json.Unmarshal([]byte(g.Secoes), &lista)
	return lista
}

// fonteDoCartao: o capítulo que a IA indicou, como está no texto, e o link dele. Sem capítulo
// (ou com um que o texto não tem), o link é o do texto inteiro.
func (g *geracao) fonteDoCartao(secao string) (string, string) {
	secao = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(secao), "## "))
	for _, s := range g.secoes() {
		if strings.EqualFold(s.Titulo, secao) {
			return s.Titulo, s.Link
		}
	}
	return "", g.Link
}

func pedidoDoTexto(g *geracao, destino, pasta string) pedidoClaude {
	if g.BaralhoID == "" && g.NomeSugerido != "" {
		destino = fmt.Sprintf("Os cartões vão para um baralho novo: use o nome %q.", g.NomeSugerido)
	}
	if lista := g.secoes(); len(lista) > 1 {
		var titulos []string
		for _, s := range lista {
			titulos = append(titulos, fmt.Sprintf("%q", s.Titulo))
		}
		destino += "\n\nCapítulos do texto (para o campo secao): " + strings.Join(titulos, ", ") + "."
	}
	if g.Origem == origemBiblioteca {
		return pedidoClaude{
			pasta: pasta,
			regra: "gerar-da-marcacao.md",
			instrucao: fmt.Sprintf("O Lucas marcou um trecho de um livro na Biblioteca e quer um cartão dele.\n\n"+
				"De onde vem: %q\n%s\n\nO trecho marcado vem junto com esta mensagem, entre <marcacao> e </marcacao>.\n\n"+
				"Quantidade: em geral um cartão; no máximo %d notas.\nTipo das notas: %s.\n%s\n\nDevolva os dados no formato pedido.",
				g.Pedido, g.Contexto, g.Quantidade, nomesDosTipos[g.Tipo], destino),
			esquema:     esquemaProposta,
			entrada:     "<marcacao>\n" + g.Texto + "\n</marcacao>\n",
			semInternet: true,
		}
	}
	if g.Tipo == "leitura" {
		return pedidoClaude{
			pasta: pasta,
			regra: "gerar-leitura.md",
			instrucao: fmt.Sprintf("O Lucas quer flashcards de leitura de um texto do Acervo, a base de conhecimento dele.\n\n"+
				"Texto: %q\n%s\n\nO texto vem junto com esta mensagem, entre <texto> e </texto>.\n\n"+
				"Quantidade: um cartão; mais só se o texto mudar de assunto, no máximo %d.\n%s\n\nDevolva os dados no formato pedido.",
				g.Pedido, g.Contexto, g.Quantidade, destino),
			esquema:     esquemaProposta,
			entrada:     "<texto>\n" + g.Texto + "\n</texto>\n",
			semInternet: true,
		}
	}
	return pedidoClaude{
		pasta: pasta,
		regra: "gerar-do-texto.md",
		instrucao: fmt.Sprintf("O Lucas quer flashcards de um texto do Acervo, a base de conhecimento dele.\n\n"+
			"Texto: %q\n%s\n\nO texto vem junto com esta mensagem, entre <texto> e </texto>.\n\n"+
			"Quantidade: só o que vale virar cartão, no máximo %d notas.\nTipo das notas: %s.\n%s\n\nDevolva os dados no formato pedido.",
			g.Pedido, g.Contexto, g.Quantidade, nomesDosTipos[g.Tipo], destino),
		esquema:     esquemaProposta,
		entrada:     "<texto>\n" + g.Texto + "\n</texto>\n",
		semInternet: true,
	}
}
