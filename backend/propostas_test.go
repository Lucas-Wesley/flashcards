package main

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"os/exec"
	"strings"
	"testing"
)

func (e *ambiente) enviarMP3(marca string) string {
	e.t.Helper()
	var corpo bytes.Buffer
	m := multipart.NewWriter(&corpo)
	f, _ := m.CreateFormFile("arquivo", "fala.mp3")
	f.Write(append([]byte("ID3\x04\x00\x00\x00\x00\x00\x00"+marca), bytes.Repeat([]byte{0xFF, 0xFB, 0x90, 0x00}, 64)...))
	m.Close()
	res, err := http.Post(e.srv.URL+"/api/audios", m.FormDataContentType(), &corpo)
	if err != nil {
		e.t.Fatal(err)
	}
	defer res.Body.Close()
	var r map[string]string
	jsonDecode(res, &r)
	if r["nome"] == "" {
		e.t.Fatalf("áudio: %d", res.StatusCode)
	}
	return r["nome"]
}

// F45: o Streaming manda as frases de um filme prontas; o áudio chega depois, antes ou depois da aprovação.
func TestPropostaProntaDoStreaming(t *testing.T) {
	e := novoAmbiente(t)
	var b baralho
	e.pedir("POST", "/api/baralhos", map[string]string{"nome": "Inglês · Filmes e séries", "tema": "Inglês"}, &b)
	fonte := func(t float64) string { return fmt.Sprintf("https://streaming.internal/#/titulo/filme/9?fala=%.1f", t) }
	corpo := map[string]any{
		"origem": "streaming", "titulo": "Frases de Filme Inventado", "link": "https://streaming.internal/#/titulo/filme/9", "baralho_id": b.ID,
		"cartoes": []map[string]any{
			{"frente": "I'll figure it out.", "verso": "Eu vou dar um jeito.\n\nfigure out: descobrir, resolver", "tags": "streaming filme-inventado", "fonte": fonte(10), "audio_pendente": true},
			{"frente": "Keep me posted.", "verso": "Me mantenha informado.", "tags": "streaming", "fonte": fonte(20), "audio_pendente": true},
			{"frente": "We ran out of time.", "verso": "Ficamos sem tempo.", "tags": "streaming", "fonte": fonte(30), "audio_pendente": true},
		},
	}
	var g geracao
	if s := e.pedir("POST", "/api/propostas", corpo, &g); s != http.StatusCreated || g.Estado != emRevisao || g.Origem != "streaming" || g.PodeRefazer {
		t.Fatalf("proposta: %d %+v", s, g)
	}
	// Mandar de novo o mesmo título, em revisão, troca a proposta.
	var de2 geracao
	e.pedir("POST", "/api/propostas", corpo, &de2)
	if de2.ID != g.ID {
		t.Errorf("criou outro pedido: %d e %d", g.ID, de2.ID)
	}
	if s := e.pedir("POST", "/api/propostas", map[string]any{"origem": "streaming", "titulo": "x", "link": "https://s/", "baralho_id": b.ID,
		"cartoes": []map[string]any{{"frente": "a", "verso": "b", "fonte": fonte(1)}, {"frente": "c", "verso": "d", "fonte": fonte(1)}}}, nil); s != http.StatusBadRequest {
		t.Errorf("fonte repetida: %d", s)
	}

	// O áudio do primeiro chega enquanto o Lucas revisa; o segundo não deu para cortar.
	audio1 := e.enviarMP3("um")
	var r map[string]int
	if s := e.pedir("POST", fmt.Sprintf("/api/geracoes/%d/audios", g.ID), map[string]any{"audios": []audioDoCartao{{Fonte: fonte(10), Audio: audio1}, {Fonte: fonte(20)}}}, &r); s != http.StatusOK || r["guardados"] != 1 {
		t.Fatalf("áudios: %d %v", s, r)
	}
	if s := e.pedir("POST", fmt.Sprintf("/api/geracoes/%d/audios", g.ID), map[string]any{"audios": []audioDoCartao{{Fonte: fonte(10), Audio: strings.Repeat("a", 32) + ".mp3"}}}, nil); s != http.StatusBadRequest {
		t.Errorf("áudio que não existe: %d", s)
	}
	var vista geracao
	e.pedir("GET", fmt.Sprintf("/api/geracoes/%d", g.ID), nil, &vista)
	c := vista.Proposta.Cartoes
	if c[0].AudioURL != "/api/audios/"+audio1 || c[0].AudioPendente || c[1].AudioPendente || c[1].AudioURL != "" || !c[2].AudioPendente {
		t.Fatalf("cartões depois dos áudios: %+v", c)
	}

	// A tela abriu antes do áudio: aprova com a cópia antiga, e o áudio que chegou entra assim mesmo.
	antigos := g.Proposta.Cartoes
	var ap map[string]any
	if s := e.pedir("POST", fmt.Sprintf("/api/geracoes/%d/aprovar", g.ID), map[string]any{"cartoes": antigos}, &ap); s != http.StatusOK {
		t.Fatalf("aprovar: %d", s)
	}
	notas := e.app.notasDoBaralho(t.Context(), b.ID)
	if len(notas) != 3 || notas[0].Audio != audio1 || notas[0].Fonte != fonte(10) || notas[2].Audio != "" {
		t.Fatalf("notas: %+v", notas)
	}
	// O terceiro chega depois de aprovado: vai para a nota.
	audio3 := e.enviarMP3("tres")
	if s := e.pedir("POST", fmt.Sprintf("/api/geracoes/%d/audios", g.ID), map[string]any{"audios": []audioDoCartao{{Fonte: fonte(30), Audio: audio3}}}, &r); s != http.StatusOK || r["guardados"] != 1 {
		t.Fatalf("áudio depois de aprovar: %d %v", s, r)
	}
	if notas := e.app.notasDoBaralho(t.Context(), b.ID); notas[2].Audio != audio3 {
		t.Errorf("nota sem o áudio que chegou depois: %+v", notas[2])
	}
	if s := e.pedir("POST", fmt.Sprintf("/api/geracoes/%d/refazer", g.ID), nil, nil); s != http.StatusConflict {
		t.Errorf("refazer o que veio pronto: %d", s)
	}
}

// pdfDeTeste monta um PDF de uma página com as linhas dadas (texto puro, Helvetica).
func pdfDeTeste(linhas ...string) []byte {
	var conteudo strings.Builder
	conteudo.WriteString("BT /F1 12 Tf 50 750 Td 16 TL\n")
	for _, l := range linhas {
		fmt.Fprintf(&conteudo, "(%s) Tj T*\n", strings.NewReplacer(`\`, `\\`, "(", `\(`, ")", `\)`).Replace(l))
	}
	conteudo.WriteString("ET")
	objetos := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", conteudo.Len(), conteudo.String()),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	}
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	var posicoes []int
	for i, o := range objetos {
		posicoes = append(posicoes, b.Len())
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objetos)+1)
	for _, p := range posicoes {
		fmt.Fprintf(&b, "%010d 00000 n \n", p)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objetos)+1, xref)
	return b.Bytes()
}

func (e *ambiente) enviarPDF(nome string, conteudo []byte, campos map[string]string) (int, geracao) {
	e.t.Helper()
	var corpo bytes.Buffer
	m := multipart.NewWriter(&corpo)
	f, _ := m.CreateFormFile("arquivo", nome)
	f.Write(conteudo)
	for k, v := range campos {
		m.WriteField(k, v)
	}
	m.Close()
	res, err := http.Post(e.srv.URL+"/api/pdfs", m.FormDataContentType(), &corpo)
	if err != nil {
		e.t.Fatal(err)
	}
	defer res.Body.Close()
	var g geracao
	if res.StatusCode < 300 {
		jsonDecode(res, &g)
	}
	return res.StatusCode, g
}

// F46: um PDF com cartões vira um pedido à IA, que lê o texto do PDF pela entrada, sem internet.
func TestCartoesDeUmPDF(t *testing.T) {
	if _, err := exec.LookPath(programaPDFParaTexto); err != nil {
		t.Skip("sem pdftotext")
	}
	e := novoAmbiente(t)
	b := e.criarBaralho("Inglês")
	s, g := e.enviarPDF("Licao 01.pdf", pdfDeTeste("Hello there, friend.", "Ola, amigo.", "   'friend' significa 'amigo'."), map[string]string{"baralho_id": b.ID})
	if s != http.StatusCreated || g.Origem != "pdf" || g.Pedido != "Licao 01" {
		t.Fatalf("enviar: %d %+v", s, g)
	}
	g = e.esperarGeracao(g.ID, emRevisao)
	obs := g.Proposta.Observacoes
	for _, quer := range []string{"Recebeu o PDF pela entrada.", "Seguiu a instrução do PDF.", "proibidas: Bash,Read,Write,Edit,Glob,Grep,Task,NotebookEdit,WebSearch,WebFetch"} {
		if !strings.Contains(obs, quer) {
			t.Errorf("falta %q em %q", quer, obs)
		}
	}
	if g.Proposta.Conferencia != nil {
		t.Error("o PDF não passa pela conferência")
	}
	if s, _ := e.enviarPDF("x.pdf", []byte("<html>não</html>"), map[string]string{"baralho_id": b.ID}); s != http.StatusBadRequest {
		t.Errorf("aceitou o que não é PDF: %d", s)
	}
	if s, _ := e.enviarPDF("vazio.pdf", pdfDeTeste(), map[string]string{"baralho_id": b.ID}); s != http.StatusBadRequest {
		t.Errorf("aceitou PDF sem texto: %d", s)
	}
	if s, _ := e.enviarPDF("y.pdf", pdfDeTeste("a"), nil); s != http.StatusBadRequest {
		t.Errorf("sem baralho: %d", s)
	}
}
