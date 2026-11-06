package main

import (
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"
)

// Flashcards de um texto do Acervo: lá, o Lucas aperta "Gerar flashcards" num Tema, num capítulo ou
// numa Referência, e o Acervo manda o texto para cá. Vira um pedido à IA como os outros (fila,
// proposta, revisão), só que os cartões saem do texto, sem a IA ir à internet. Os do mesmo Tema vão
// para o mesmo baralho: o Acervo sugere o nome, e um baralho com esse nome recebe os cartões novos e
// atualiza os de mesma frente.

const origemAcervo = "acervo"

// Um Tema inteiro pode ser grande; acima disso o Acervo pede para mandar por capítulo.
const tamanhoMaximoDoTexto = 150_000

func (a *app) handlePedirDoTexto(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4*tamanhoMaximoDoTexto)
	var corpo struct {
		Titulo   string `json:"titulo"`   // "Tema › Capítulo"
		Baralho  string `json:"baralho"`  // nome sugerido: o do Tema principal
		Contexto string `json:"contexto"` // de onde vem o texto, para a IA entender
		Texto    string `json:"texto"`
		Link     string `json:"link"` // o texto no Acervo
		Tipo     string `json:"tipo"`
	}
	if !lerCorpo(w, r, &corpo) {
		return
	}
	corpo.Titulo, corpo.Baralho, corpo.Texto = strings.TrimSpace(corpo.Titulo), strings.TrimSpace(corpo.Baralho), strings.TrimSpace(corpo.Texto)
	if corpo.Tipo == "" {
		corpo.Tipo = "basico"
	}
	tamanho := utf8.RuneCountInString(corpo.Texto)
	switch {
	case corpo.Titulo == "":
		erroJSON(w, http.StatusBadRequest, "Diga de onde vem o texto.")
		return
	case tamanho < 200:
		erroJSON(w, http.StatusBadRequest, "O texto é curto demais para virar cartões.")
		return
	case tamanho > tamanhoMaximoDoTexto:
		erroJSON(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("O texto tem %d mil caracteres; o máximo é %d mil. Gere por capítulo.", tamanho/1000, tamanhoMaximoDoTexto/1000))
		return
	case tiposDeNota[corpo.Tipo] == 0:
		erroJSON(w, http.StatusBadRequest, "Tipo de nota inválido.")
		return
	}
	// Já existe o baralho do Tema: os cartões vão para ele.
	var baralhoID string
	a.db.QueryRowContext(r.Context(), `SELECT id FROM baralhos WHERE nome = ? ORDER BY criado_em LIMIT 1`, corpo.Baralho).Scan(&baralhoID)

	res, err := a.db.ExecContext(r.Context(), `INSERT INTO geracoes (origem, pedido, texto, contexto, link, nome_sugerido, quantidade, tipo, baralho_id, estado, criado_em)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		origemAcervo, corpo.Titulo, corpo.Texto, strings.TrimSpace(corpo.Contexto), strings.TrimSpace(corpo.Link), corpo.Baralho,
		limiteDeCartoes(corpo.Texto), corpo.Tipo, baralhoID, naFila, a.agora())
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
func limiteDeCartoes(texto string) int {
	return min(100, max(5, len(strings.Fields(texto))/80))
}

func pedidoDoTexto(g *geracao, destino, pasta string) pedidoClaude {
	if g.BaralhoID == "" && g.NomeSugerido != "" {
		destino = fmt.Sprintf("Os cartões vão para um baralho novo: use o nome %q.", g.NomeSugerido)
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
