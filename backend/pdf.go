package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// Cartões de um PDF (F46). O Lucas sobe um PDF que já traz cartões (frase em inglês e a tradução,
// frente e verso, listas de frases, como os de um curso), e eles viram uma proposta para revisar.
// O texto sai do PDF pelo pdftotext (poppler), mantendo a disposição das linhas (o recuo separa a
// frase da nota que a explica); a IA lê esse texto pela entrada padrão, sem internet, e devolve os
// cartões como estão no PDF, seguindo regras/extrair-do-pdf.md (editável em Ajustes). Não inventa
// frase nem tradução. Na fila e na revisão de sempre, com "Refazer" e "Pedir algo a mais".

const origemPDF = "pdf"

const tamanhoMaximoDoPDF = 50 << 20

var programaPDFParaTexto = "pdftotext"

// modeloDoPDF: no teste com um PDF de curso (F46), o Haiku e o Sonnet tiraram os mesmos cartões, sem
// inventar nada; o Haiku juntou as frases partidas em linhas, como a instrução pede, e gasta bem menos.
func modeloDoPDF() string { return primeiro(os.Getenv("MODELO_DO_PDF"), "haiku") }

func (a *app) handlePedirDoPDF(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, tamanhoMaximoDoPDF+1<<20)
	arquivo, cabecalho, err := r.FormFile("arquivo")
	if err != nil {
		erroJSON(w, http.StatusBadRequest, "Envie um PDF de até 50 MB.")
		return
	}
	defer arquivo.Close()
	baralhoID := strings.TrimSpace(r.FormValue("baralho_id"))
	nomeNovo := strings.TrimSpace(r.FormValue("baralho"))
	tipo := r.FormValue("tipo")
	if tipo == "" {
		tipo = "basico"
	}
	instrucao := strings.TrimSpace(r.FormValue("instrucao"))
	switch {
	case !tipoDePergunta(tipo):
		erroJSON(w, http.StatusBadRequest, "Tipo de nota inválido.")
		return
	case baralhoID != "" && !a.existeBaralho(r.Context(), baralhoID):
		erroJSON(w, http.StatusBadRequest, "Esse baralho não existe mais. Escolha outro.")
		return
	case baralhoID == "" && nomeNovo == "":
		erroJSON(w, http.StatusBadRequest, "Escolha um baralho ou dê o nome do baralho novo.")
		return
	case utf8.RuneCountInString(instrucao) > maximoDaInstrucao:
		erroJSON(w, http.StatusBadRequest, fmt.Sprintf("A instrução vai até %d caracteres.", maximoDaInstrucao))
		return
	}
	if baralhoID != "" {
		nomeNovo = ""
	}
	texto, err := textoDoPDF(r.Context(), arquivo)
	if err != nil {
		erroJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	tamanho := utf8.RuneCountInString(texto)
	if tamanho > tamanhoMaximoDoTexto {
		erroJSON(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("O texto do PDF tem %d mil caracteres; o máximo é %d mil. Divida o PDF em partes.", tamanho/1000, tamanhoMaximoDoTexto/1000))
		return
	}
	nome := strings.TrimSpace(strings.TrimSuffix(filepath.Base(cabecalho.Filename), filepath.Ext(cabecalho.Filename)))
	if nome == "" {
		nome = "PDF"
	}
	res, err := a.db.ExecContext(r.Context(), `INSERT INTO geracoes (origem, pedido, instrucao, texto, nome_sugerido, quantidade, tipo, baralho_id, estado, criado_em)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		origemPDF, nome, instrucao, texto, nomeNovo, maximoDeCartoes, tipo, baralhoID, naFila, a.agora())
	if err != nil {
		falha(w, err)
		return
	}
	id, _ := res.LastInsertId()
	a.fila.avisar()
	a.responderGeracao(w, r, http.StatusCreated, id)
}

var (
	reEspacosNoMeio = regexp.MustCompile(`(\S) {3,}`)
	reLinhasVazias  = regexp.MustCompile(`\n{3,}`)
)

// textoDoPDF tira o texto do PDF (com a disposição das linhas) e marca o começo de cada página.
// Sem texto (um PDF só de imagens, escaneado), recusa.
func textoDoPDF(ctx context.Context, r io.Reader) (string, error) {
	tmp, err := os.CreateTemp("", "pdf-*.pdf")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	cabeca := make([]byte, 5)
	n, _ := io.ReadFull(r, cabeca)
	if string(cabeca[:n]) != "%PDF-" {
		tmp.Close()
		return "", errors.New("Esse arquivo não é um PDF.")
	}
	tmp.Write(cabeca[:n])
	if _, err := io.Copy(tmp, r); err != nil {
		tmp.Close()
		return "", err
	}
	tmp.Close()
	c, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	saida, err := exec.CommandContext(c, programaPDFParaTexto, "-layout", "-enc", "UTF-8", tmp.Name(), "-").Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return "", errors.New("Não consegui ler esse PDF (está protegido ou corrompido?).")
		}
		return "", fmt.Errorf("leitor de PDF: %w", err)
	}
	if len(strings.Fields(string(saida))) < 3 {
		return "", errors.New("Esse PDF não tem texto para ler (parece ser só imagem, escaneado).")
	}
	var paginas []string
	for i, p := range strings.Split(string(saida), "\f") {
		var linhas []string
		for _, l := range strings.Split(p, "\n") {
			linhas = append(linhas, reEspacosNoMeio.ReplaceAllString(strings.TrimRight(l, " \t\r"), "$1   "))
		}
		p = strings.TrimSpace(reLinhasVazias.ReplaceAllString(strings.Join(linhas, "\n"), "\n\n"))
		if p != "" {
			paginas = append(paginas, fmt.Sprintf("--- página %d ---\n%s", i+1, p))
		}
	}
	return strings.Join(paginas, "\n\n"), nil
}

func pedidoDoPDF(g *geracao, destino, pasta string) pedidoClaude {
	if g.BaralhoID == "" && g.NomeSugerido != "" {
		destino = fmt.Sprintf("Os cartões vão para um baralho novo: use o nome %q.", g.NomeSugerido)
	}
	return pedidoClaude{
		pasta: pasta,
		regra: "extrair-do-pdf.md",
		instrucao: fmt.Sprintf("O Lucas subiu um PDF que traz cartões (frases com tradução, frente e verso, listas) e quer esses cartões no app.\n\n"+
			"Arquivo: %q\n\nO texto do PDF vem junto com esta mensagem, entre <pdf> e </pdf>, página por página, com a disposição das linhas.\n\n"+
			"Quantidade: todos os cartões que o PDF traz, sem repetir (no máximo %d notas).\nTipo das notas: %s.\n%s\n\nDevolva os dados no formato pedido.",
			g.Pedido, g.Quantidade, nomesDosTipos[g.Tipo], destino),
		esquema:     esquemaProposta,
		entrada:     "<pdf>\n" + g.Texto + "\n</pdf>\n",
		semInternet: true,
		modelo:      modeloDoPDF(),
	}
}
