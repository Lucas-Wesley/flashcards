package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// O Claude Code roda headless (claude -p), como no Acervo: reaproveita a assinatura, e o login vem
// do CLAUDE_CODE_OAUTH_TOKEN, que fica em /opt/apps/flashcards.env, fora do Git.
//
// Gerar um baralho só precisa da internet, para conferir os fatos; arquivos e terminal ficam proibidos.
const (
	ferramentasInternet  = "WebSearch,WebFetch"
	ferramentasProibidas = "Bash,Read,Write,Edit,Glob,Grep,Task,NotebookEdit"
)

type pedidoClaude struct {
	pasta     string // diretório de trabalho (vazio, só da tarefa)
	regra     string // arquivo em regras/; vale a versão editada no app, se houver
	instrucao string // o pedido específico desta tarefa
	esquema   string // JSON Schema da resposta
}

type respostaClaude struct {
	Resultado   string          `json:"result"`
	Erro        bool            `json:"is_error"`
	Estruturado json.RawMessage `json:"structured_output"`
}

func (a *app) chamarClaude(ctx context.Context, p pedidoClaude) (respostaClaude, error) {
	var r respostaClaude
	regra, _, err := a.lerRegra(p.regra)
	if err != nil {
		return r, fmt.Errorf("instrução %s não encontrada: %w", p.regra, err)
	}
	prompt := p.instrucao + "\n\nSiga as instruções abaixo.\n\n" + regra

	// Sem guardar a sessão: cada geração deixaria um histórico em $HOME/.claude que ninguém lê.
	args := []string{"-p", prompt, "--output-format", "json", "--no-session-persistence",
		"--allowedTools", ferramentasInternet, "--disallowedTools", ferramentasProibidas}
	if p.esquema != "" {
		args = append(args, "--json-schema", p.esquema)
	}
	if a.cfg.modelo != "" {
		args = append(args, "--model", a.cfg.modelo)
	}

	cmd := exec.CommandContext(ctx, a.cfg.claude, args...)
	cmd.Dir = p.pasta
	// Cancelado, o claude morre na hora; não esperar mais que isso por filhos que seguram a saída.
	cmd.WaitDelay = 5 * time.Second
	var saida, erros bytes.Buffer
	cmd.Stdout, cmd.Stderr = &saida, &erros
	errExec := cmd.Run()
	if ctx.Err() != nil {
		return r, ctx.Err()
	}
	if jerr := json.Unmarshal(saida.Bytes(), &r); jerr != nil {
		msg := primeiraLinha(erros.String(), primeiraLinha(saida.String(), "sem resposta"))
		if errExec != nil && msg == "sem resposta" {
			msg = errExec.Error()
		}
		if errExec != nil {
			return r, fmt.Errorf("claude falhou: %s", msg)
		}
		return r, fmt.Errorf("resposta do claude ilegível: %s", msg)
	}
	if r.Erro || errExec != nil {
		msg := primeiraLinha(r.Resultado, primeiraLinha(erros.String(), "erro sem mensagem"))
		if strings.Contains(msg, "Not logged in") || strings.Contains(msg, "/login") {
			return r, errors.New("O Claude não está autenticado neste servidor: falta o CLAUDE_CODE_OAUTH_TOKEN (gerado com \"claude setup-token\").")
		}
		return r, fmt.Errorf("claude falhou: %s", msg)
	}
	return r, nil
}

// dadosDaResposta lê a resposta estruturada; sem ela, procura um JSON no texto.
func dadosDaResposta(r respostaClaude, destino any) error {
	if len(r.Estruturado) > 0 && string(r.Estruturado) != "null" {
		return json.Unmarshal(r.Estruturado, destino)
	}
	texto := r.Resultado
	ini, fim := strings.Index(texto, "{"), strings.LastIndex(texto, "}")
	if ini < 0 || fim < ini {
		return fmt.Errorf("o claude não devolveu dados: %s", primeiraLinha(texto, "resposta vazia"))
	}
	return json.Unmarshal([]byte(texto[ini:fim+1]), destino)
}

func primeiraLinha(s, padrao string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return padrao
	}
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > 300 {
		s = s[:300] + "…"
	}
	return s
}
