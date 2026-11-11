package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

// A conferência (F40): depois de a IA montar os cartões e o app buscar as imagens, uma segunda
// chamada, com outra instrução (regras/conferir-baralho.md), lê o pedido e a proposta pronta, com o
// nome de cada imagem que veio, e aponta o que não atende ao pedido: um item do conjunto que
// faltou, uma frente que entrega a resposta, uma imagem de outra coisa. Ela não muda nada; os
// avisos aparecem na revisão, em cada cartão, para o Lucas decidir. Se a conferência falhar, a
// proposta segue sem ela, com o motivo.

// tempoDaConferencia: ler 200 cartões e pesquisar uma ou outra dúvida cabe nisso; o que passar
// não segura a proposta.
const tempoDaConferencia = 8 * time.Minute

type conferencia struct {
	Resumo   string   `json:"resumo,omitempty"`
	Faltando []string `json:"faltando,omitempty"`
	// Falhou: a conferência não rodou (o motivo); a proposta vale do mesmo jeito.
	Falhou string `json:"falhou,omitempty"`
}

type respostaDaConferencia struct {
	Resumo    string   `json:"resumo"`
	Faltando  []string `json:"faltando"`
	Problemas []struct {
		Cartao   int    `json:"cartao"`
		Problema string `json:"problema"`
	} `json:"problemas"`
}

const esquemaConferencia = `{
  "type": "object",
  "properties": {
    "resumo": {"type": "string"},
    "faltando": {"type": "array", "items": {"type": "string"}},
    "problemas": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "cartao": {"type": "integer"},
          "problema": {"type": "string"}
        },
        "required": ["cartao", "problema"]
      }
    }
  },
  "required": ["resumo", "problemas"]
}`

// cartoesParaConferir: a proposta como a conferência lê, numerada a partir de 1, com o que foi
// pedido de cada imagem e o arquivo que veio.
func cartoesParaConferir(cartoes []cartaoProposto) string {
	var b strings.Builder
	b.WriteString("<cartoes>\n")
	for i, c := range cartoes {
		fmt.Fprintf(&b, "\n[%d]\nFrente: %s\nVerso: %s\n", i+1, c.Frente, c.Verso)
		for _, im := range []struct{ lado, pedida, veio string }{{"da frente", c.ImagemFrente, c.tituloImagemFrente}, {"do verso", c.Imagem, c.tituloImagem}} {
			if im.pedida == "" && im.veio == "" {
				continue
			}
			veio := im.veio
			if veio == "" {
				veio = "nenhuma (o cartão fica sem)"
			}
			fmt.Fprintf(&b, "Imagem %s: pedida %q; veio %s\n", im.lado, im.pedida, veio)
		}
	}
	b.WriteString("\n</cartoes>\n")
	return b.String()
}

func (a *app) conferir(ctx context.Context, g *geracao, p *proposta, pasta string) {
	ctx, cancel := context.WithTimeout(ctx, tempoDaConferencia)
	defer cancel()
	quantidade := "não disse quantos"
	if g.Quantidade > 0 {
		quantidade = fmt.Sprintf("sugeriu cerca de %d (uma referência, não um teto)", g.Quantidade)
	}
	r, err := a.chamarClaude(ctx, pedidoClaude{
		pasta: pasta,
		regra: "conferir-baralho.md",
		instrucao: fmt.Sprintf("Confira a proposta de flashcards que outra IA montou para o Lucas.\n\nPedido: %q\nQuantidade: %s.\n"+
			"Tipo das notas: %s.\n\nComo a IA entendeu: %q\nObservações dela: %q\n\nOs %d cartões vêm junto com esta mensagem, entre "+
			"<cartoes> e </cartoes>.\n\nDevolva os dados no formato pedido.",
			g.Pedido, quantidade, nomesDosTipos[g.Tipo], p.Entendimento, p.Observacoes, len(p.Cartoes)),
		esquema: esquemaConferencia,
		entrada: cartoesParaConferir(p.Cartoes),
	})
	var c respostaDaConferencia
	if err == nil {
		err = dadosDaResposta(r, &c)
	}
	if err != nil {
		if ctx.Err() != nil {
			err = fmt.Errorf("passou de %d minutos", int(tempoDaConferencia.Minutes()))
		}
		log.Printf("geração %d: conferência: %v", g.ID, err)
		p.Conferencia = &conferencia{Falhou: err.Error()}
		return
	}
	p.Conferencia = &conferencia{Resumo: strings.TrimSpace(c.Resumo)}
	for _, f := range c.Faltando {
		if f = strings.TrimSpace(f); f != "" {
			p.Conferencia.Faltando = append(p.Conferencia.Faltando, f)
		}
	}
	for _, pr := range c.Problemas {
		if pr.Cartao >= 1 && pr.Cartao <= len(p.Cartoes) && strings.TrimSpace(pr.Problema) != "" {
			p.Cartoes[pr.Cartao-1].Avisos = append(p.Cartoes[pr.Cartao-1].Avisos, strings.TrimSpace(pr.Problema))
		}
	}
}
