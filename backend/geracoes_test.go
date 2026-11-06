package main

import (
	"bytes"
	"context"
	"database/sql"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

func (e *ambiente) pedirGeracao(corpo map[string]any) geracao {
	e.t.Helper()
	var g geracao
	if s := e.pedir("POST", "/api/geracoes", corpo, &g); s != http.StatusCreated {
		e.t.Fatalf("pedir geração: %d", s)
	}
	return g
}

func (e *ambiente) esperarGeracao(id int64, estado string) geracao {
	e.t.Helper()
	var g geracao
	for range 200 {
		e.pedir("GET", "/api/geracoes/"+strconv.FormatInt(id, 10), nil, &g)
		if g.Estado == estado {
			return g
		}
		time.Sleep(25 * time.Millisecond)
	}
	e.t.Fatalf("geração %d ficou em %q (erro %q), esperava %q", id, g.Estado, g.Erro, estado)
	return g
}

func TestGerarBaralhoNovoComIA(t *testing.T) {
	e := novoAmbiente(t)
	g := e.pedirGeracao(map[string]any{"pedido": "presidentes do Brasil no século 21", "quantidade": 10})
	g = e.esperarGeracao(g.ID, emRevisao)

	p := g.Proposta
	if p.NomeBaralho != "Presidentes do Brasil no século 21" || len(p.Cartoes) != 2 {
		t.Fatalf("proposta: %+v", p)
	}
	// Só a internet: arquivos e terminal ficam proibidos.
	if !strings.Contains(p.Observacoes, "Ferramentas: WebSearch,WebFetch;") || !strings.Contains(p.Observacoes, "Bash") {
		t.Errorf("ferramentas: %q", p.Observacoes)
	}
	// A foto vem do artigo certo (o primeiro da busca com foto), e só a URL: nada baixado ainda.
	if !strings.HasSuffix(p.Cartoes[0].ImagemURL, "/foto.png") || p.Cartoes[1].ImagemURL != "" {
		t.Fatalf("imagens: %+v", p.Cartoes)
	}
	if arquivos, _ := os.ReadDir(filepath.Join(e.dados, "media")); len(arquivos) != 0 {
		t.Fatalf("baixou imagem antes da aprovação: %d", len(arquivos))
	}

	// O Lucas edita um cartão e aprova os dois.
	p.Cartoes[1].Verso = "Dilma Rousseff (2011–2016)"
	var res struct {
		BaralhoID string `json:"baralho_id"`
		Criadas   int    `json:"criadas"`
		SemImagem int    `json:"sem_imagem"`
	}
	corpo := map[string]any{"nome_baralho": "Presidentes", "cartoes": p.Cartoes}
	if s := e.pedir("POST", "/api/geracoes/"+strconv.FormatInt(g.ID, 10)+"/aprovar", corpo, &res); s != http.StatusOK {
		t.Fatalf("aprovar: %d", s)
	}
	if res.Criadas != 2 || res.SemImagem != 0 {
		t.Fatalf("aprovado: %+v", res)
	}
	var b resumo
	e.pedir("GET", "/api/baralhos/"+res.BaralhoID, nil, &b)
	if b.Nome != "Presidentes" || b.Cartoes != 2 {
		t.Fatalf("baralho: %+v", b)
	}
	var notas []nota
	e.pedir("GET", "/api/notas?q=Dilma", nil, &notas)
	if len(notas) != 1 || notas[0].Verso != "Dilma Rousseff (2011–2016)" || notas[0].Imagem != "" {
		t.Fatalf("nota editada: %+v", notas)
	}
	e.pedir("GET", "/api/notas?q=2003", nil, &notas)
	if len(notas) != 1 || !nomeDeImagem.MatchString(notas[0].Imagem) {
		t.Fatalf("nota com foto: %+v", notas)
	}
	foto, err := http.Get(e.srv.URL + "/api/imagens/" + notas[0].Imagem)
	if err != nil || foto.StatusCode != http.StatusOK || foto.Header.Get("Content-Type") != "image/png" {
		t.Fatalf("foto: %v %v", err, foto)
	}
	foto.Body.Close()
	if es := e.estudo(res.BaralhoID); es.Cartao.Imagem != notas[0].Imagem {
		t.Errorf("o estudo devia mostrar a foto: %+v", es.Cartao)
	}

	if s := e.pedir("POST", "/api/geracoes/"+strconv.FormatInt(g.ID, 10)+"/aprovar", corpo, nil); s != http.StatusConflict {
		t.Errorf("aprovar duas vezes: %d", s)
	}
}

func TestGerarNumBaralhoExistenteComInstrucaoEditada(t *testing.T) {
	e := novoAmbiente(t)
	b := e.criarBaralho("História")
	e.criarNota(b.ID, "basico", "Quem foi o primeiro presidente?", "Deodoro")

	var i instrucao
	e.pedir("GET", "/api/instrucoes/gerar-baralho.md", nil, &i)
	if !strings.Contains(i.Conteudo, "Um fato por cartão") || i.Editada {
		t.Fatalf("instrução padrão: %+v", i)
	}
	e.pedir("PUT", "/api/instrucoes/gerar-baralho.md", map[string]string{"conteudo": i.Conteudo + "\nREGRA EDITADA\n"}, &i)
	if !i.Editada {
		t.Fatal("instrução não ficou editada")
	}

	g := e.pedirGeracao(map[string]any{"pedido": "presidentes", "quantidade": 5, "tipo": "invertido", "baralho_id": b.ID})
	g = e.esperarGeracao(g.ID, emRevisao)
	if !strings.Contains(g.Proposta.Observacoes, "Seguiu a instrução editada.") || !strings.Contains(g.Proposta.Observacoes, "Recebeu os cartões existentes.") {
		t.Fatalf("a IA não recebeu a instrução editada ou os cartões existentes: %q", g.Proposta.Observacoes)
	}

	// Aprova só o primeiro cartão, no baralho que já existe, nos dois sentidos.
	var res struct {
		BaralhoID string `json:"baralho_id"`
	}
	e.pedir("POST", "/api/geracoes/"+strconv.FormatInt(g.ID, 10)+"/aprovar", map[string]any{"cartoes": g.Proposta.Cartoes[:1]}, &res)
	var r resumo
	e.pedir("GET", "/api/baralhos/"+b.ID, nil, &r)
	if res.BaralhoID != b.ID || r.Cartoes != 3 {
		t.Fatalf("baralho existente: %q %+v", res.BaralhoID, r)
	}
}

func TestGeracaoQueFalhaPodeSerRefeitaECancelada(t *testing.T) {
	e := novoAmbiente(t)
	t.Setenv("CLAUDE_FALSO_FALHA", "1")
	g := e.pedirGeracao(map[string]any{"pedido": "capitais", "quantidade": 5})
	g = e.esperarGeracao(g.ID, falhou)
	if !strings.Contains(g.Erro, "limite de uso atingido") {
		t.Fatalf("erro: %q", g.Erro)
	}
	t.Setenv("CLAUDE_FALSO_FALHA", "")
	t.Setenv("CLAUDE_FALSO_ESPERA", "5")
	id := strconv.FormatInt(g.ID, 10)
	if s := e.pedir("POST", "/api/geracoes/"+id+"/refazer", nil, nil); s != http.StatusOK {
		t.Fatalf("refazer: %d", s)
	}
	e.esperarGeracao(g.ID, rodando)
	if s := e.pedir("POST", "/api/geracoes/"+id+"/cancelar", nil, nil); s != http.StatusOK {
		t.Fatalf("cancelar: %d", s)
	}
	e.esperarGeracao(g.ID, cancelada)
	if s := e.pedir("POST", "/api/geracoes/"+id+"/descartar", nil, nil); s != http.StatusConflict {
		t.Errorf("descartar o que não é proposta: %d", s)
	}

	if s := e.pedir("POST", "/api/geracoes", map[string]any{"pedido": " ", "quantidade": 5}, nil); s != http.StatusBadRequest {
		t.Errorf("pedido vazio aceito: %d", s)
	}
	if s := e.pedir("POST", "/api/geracoes", map[string]any{"pedido": "x", "quantidade": 500}, nil); s != http.StatusBadRequest {
		t.Errorf("500 cartões aceitos: %d", s)
	}
}

func TestBuscaDeImagemOrdenaPeloCartao(t *testing.T) {
	e := novoAmbiente(t)
	q := url.Values{"q": {"Bandeira do Brasil"}}
	var achadas []imagemAchada
	e.pedir("GET", "/api/imagens?"+q.Encode(), nil, &achadas)
	var titulos []string
	for _, a := range achadas {
		titulos = append(titulos, a.Fonte+": "+a.Titulo)
	}
	// Sem contexto, vale o termo (no empate, a ordem das fontes); o PDF do Commons fica de fora.
	esperado := []string{"Wikipédia: Bandeira do Brasil", "Wikimedia Commons: Flag of Brazil", "Wikipédia: Bandeira do Império do Brasil", "Openverse · flickr: Bandeira na praia"}
	if !slices.Equal(titulos, esperado) {
		t.Fatalf("sem contexto: %v", titulos)
	}
	if achadas[1].Autor != "Fulano" || achadas[3].Licenca != "CC BY-NC 2.0" {
		t.Errorf("crédito: %+v", achadas)
	}

	// Num cartão sobre 1822 e o Império, a bandeira da época vem antes da atual.
	q.Set("contexto", "Por que a bandeira do Brasil mudou em 1822?\nPor causa da Independência: as armas do novo Império.")
	achadas = nil
	e.pedir("GET", "/api/imagens?"+q.Encode(), nil, &achadas)
	titulos = nil
	for _, a := range achadas {
		titulos = append(titulos, a.Titulo)
	}
	// O ano do cartão também vai para o Commons, que acha a bandeira de 1822.
	esperado = []string{"Flag of Brazil (1822–1853)", "Bandeira do Império do Brasil", "Bandeira do Brasil", "Flag of Brazil", "Bandeira na praia"}
	if !slices.Equal(titulos, esperado) {
		t.Fatalf("com contexto: %v", titulos)
	}
}

func TestImagens(t *testing.T) {
	e := novoAmbiente(t)

	var achadas []imagemAchada
	e.pedir("GET", "/api/imagens?q=Lula", nil, &achadas)
	if len(achadas) != 1 || achadas[0].Titulo != "Luiz Inácio Lula da Silva" {
		t.Fatalf("busca: %+v", achadas)
	}

	var baixada map[string]string
	if s := e.pedir("POST", "/api/imagens/baixar", map[string]string{"url": achadas[0].URL}, &baixada); s != http.StatusOK {
		t.Fatalf("baixar: %d", s)
	}
	if s := e.pedir("POST", "/api/imagens/baixar", map[string]string{"url": "https://exemplo.com/x.png"}, nil); s != http.StatusBadRequest {
		t.Errorf("baixou de fora da Wikipédia: %d", s)
	}

	enviar := func(conteudo []byte) (int, string) {
		var corpo bytes.Buffer
		m := multipart.NewWriter(&corpo)
		f, _ := m.CreateFormFile("arquivo", "foto.png")
		f.Write(conteudo)
		m.Close()
		res, err := http.Post(e.srv.URL+"/api/imagens", m.FormDataContentType(), &corpo)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var r map[string]string
		jsonDecode(res, &r)
		return res.StatusCode, r["nome"]
	}
	// O mesmo conteúdo dá o mesmo nome: não guarda duas vezes.
	if s, nome := enviar(pngDeTeste); s != http.StatusOK || nome != baixada["nome"] {
		t.Fatalf("enviar: %d %q (baixada %q)", s, nome, baixada["nome"])
	}
	if s, _ := enviar([]byte("<svg onload=alert(1)></svg>")); s != http.StatusBadRequest {
		t.Errorf("aceitou o que não é imagem: %d", s)
	}

	b := e.criarBaralho("Geral")
	if s := e.pedir("POST", "/api/notas", dadosDaNota{BaralhoID: b.ID, Frente: "x", Imagem: "../main.go"}, nil); s != http.StatusBadRequest {
		t.Errorf("nota com imagem inválida aceita: %d", s)
	}
	var n nota
	e.pedir("POST", "/api/notas", dadosDaNota{BaralhoID: b.ID, Frente: "x", Imagem: baixada["nome"]}, &n)
	if n.Imagem != baixada["nome"] {
		t.Fatalf("nota com imagem: %+v", n)
	}
	if res, _ := http.Get(e.srv.URL + "/api/imagens/..%2Fflashcards.db"); res.StatusCode != http.StatusNotFound {
		t.Errorf("arquivo fora de media: %d", res.StatusCode)
	}
}

// Um banco da primeira versão, sem a coluna de imagem, ganha a coluna ao abrir.
func TestMigraBancoAntigo(t *testing.T) {
	caminho := filepath.Join(t.TempDir(), "antigo.db")
	db, err := sql.Open("sqlite", caminho)
	if err != nil {
		t.Fatal(err)
	}
	db.Exec(`CREATE TABLE notas (id TEXT PRIMARY KEY, baralho_id TEXT NOT NULL, tipo TEXT NOT NULL, frente TEXT NOT NULL,
		verso TEXT NOT NULL, tags TEXT NOT NULL DEFAULT '', criada_em TEXT NOT NULL, atualizada_em TEXT NOT NULL)`)
	db.Exec(`INSERT INTO notas VALUES ('n1', 'b1', 'basico', 'f', 'v', '', '', '')`)
	db.Close()

	db, err = abrirBanco(caminho)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var imagem string
	if err := db.QueryRow(`SELECT imagem || audio FROM notas WHERE id = 'n1'`).Scan(&imagem); err != nil || imagem != "" {
		t.Fatalf("migração: %v %q", err, imagem)
	}
}

// O pedido aprovado é editado e refeito: a IA trabalha no baralho criado, recebe os cartões que já
// existem, e aprovar de novo atualiza os de mesma frente (com o histórico e a foto) em vez de duplicar.
func TestRefazerPedidoAprovadoAtualizaOsCartoes(t *testing.T) {
	e := novoAmbiente(t)
	g := e.pedirGeracao(map[string]any{"pedido": "presidentes", "quantidade": 5})
	g = e.esperarGeracao(g.ID, emRevisao)
	id := strconv.FormatInt(g.ID, 10)
	var res struct {
		BaralhoID   string `json:"baralho_id"`
		Criadas     int    `json:"criadas"`
		Atualizadas int    `json:"atualizadas"`
	}
	e.pedir("POST", "/api/geracoes/"+id+"/aprovar", map[string]any{"nome_baralho": "Presidentes", "cartoes": g.Proposta.Cartoes}, &res)
	if res.Criadas != 2 {
		t.Fatalf("primeira aprovação: %+v", res)
	}
	es := e.responder(e.estudo(res.BaralhoID), 3)
	var antes []nota
	e.pedir("GET", "/api/notas?q=2003", nil, &antes)

	if s := e.pedir("POST", "/api/geracoes/"+id+"/refazer", map[string]any{"pedido": "presidentes PEDIDO EDITADO", "quantidade": 3}, &g); s != http.StatusOK {
		t.Fatalf("refazer o aprovado: %d", s)
	}
	if g.Pedido != "presidentes PEDIDO EDITADO" || g.Quantidade != 3 || g.BaralhoID != res.BaralhoID {
		t.Fatalf("pedido refeito: %+v", g)
	}
	g = e.esperarGeracao(g.ID, emRevisao)
	if !strings.Contains(g.Proposta.Observacoes, "Recebeu o pedido editado.") || !strings.Contains(g.Proposta.Observacoes, "Recebeu os cartões existentes.") {
		t.Fatalf("observações: %q", g.Proposta.Observacoes)
	}
	c := g.Proposta.Cartoes[0]
	if !c.Atualiza || c.ImagemURL != "/api/imagens/"+antes[0].Imagem {
		t.Fatalf("o cartão existente devia vir como atualização, com a foto que tem: %+v", c)
	}

	// Refazer com a proposta ainda em revisão também vale (ela é trocada).
	if s := e.pedir("POST", "/api/geracoes/"+id+"/refazer", nil, &g); s != http.StatusOK {
		t.Fatalf("refazer em revisão: %d", s)
	}
	g = e.esperarGeracao(g.ID, emRevisao)

	g.Proposta.Cartoes[0].Verso = "Lula: desenhou a bandeira em 1889"
	e.pedir("POST", "/api/geracoes/"+id+"/aprovar", map[string]any{"cartoes": g.Proposta.Cartoes}, &res)
	if res.Criadas != 0 || res.Atualizadas != 2 {
		t.Fatalf("segunda aprovação: %+v", res)
	}
	var depois []nota
	e.pedir("GET", "/api/notas?q=2003", nil, &depois)
	if len(depois) != 1 || depois[0].ID != antes[0].ID || depois[0].Verso != "Lula: desenhou a bandeira em 1889" || depois[0].Imagem != antes[0].Imagem {
		t.Fatalf("nota atualizada: %+v", depois)
	}
	var b resumo
	e.pedir("GET", "/api/baralhos/"+res.BaralhoID, nil, &b)
	if b.Cartoes != 2 || es.FeitosHoje != 1 || b.FeitosHoje != 1 {
		t.Fatalf("o histórico devia ficar: %+v", b)
	}

	var doBaralho []geracao
	e.pedir("GET", "/api/geracoes?baralho="+res.BaralhoID, nil, &doBaralho)
	if len(doBaralho) != 1 || doBaralho[0].ID != g.ID {
		t.Fatalf("pedidos do baralho: %+v", doBaralho)
	}
}

func TestLimpezaApagaSoAMidiaSemUso(t *testing.T) {
	e := novoAmbiente(t)
	g := e.pedirGeracao(map[string]any{"pedido": "presidentes do Brasil no século 21", "quantidade": 10})
	g = e.esperarGeracao(g.ID, emRevisao)
	var res struct {
		BaralhoID string `json:"baralho_id"`
	}
	corpo := map[string]any{"nome_baralho": "Presidentes", "cartoes": g.Proposta.Cartoes}
	if s := e.pedir("POST", "/api/geracoes/"+strconv.FormatInt(g.ID, 10)+"/aprovar", corpo, &res); s != http.StatusOK {
		t.Fatalf("aprovar: %d", s)
	}
	var notas []nota
	e.pedir("GET", "/api/notas?q=2003", nil, &notas)
	usada := notas[0].Imagem

	pasta := filepath.Join(e.dados, "media")
	velho := time.Now().Add(-48 * time.Hour)
	for _, nome := range []string{"0123456789abcdef0123456789abcdef.jpg", "fedcba9876543210fedcba9876543210.mp3", "x.jpg.tmp", "leia-me.txt"} {
		os.WriteFile(filepath.Join(pasta, nome), []byte("x"), 0o644)
		os.Chtimes(filepath.Join(pasta, nome), velho, velho)
	}
	os.WriteFile(filepath.Join(pasta, "abcdefabcdefabcdefabcdefabcdefab.png"), pngDeTeste, 0o644) // recém-enviada no editor
	os.Chtimes(filepath.Join(pasta, usada), velho, velho)

	n, err := e.app.limparMidia(context.Background(), time.Now().Add(-carenciaDaMidia))
	if err != nil || n != 3 {
		t.Fatalf("limpeza: %d %v", n, err)
	}
	var sobraram []string
	arquivos, _ := os.ReadDir(pasta)
	for _, a := range arquivos {
		sobraram = append(sobraram, a.Name())
	}
	slices.Sort(sobraram)
	esperado := []string{"abcdefabcdefabcdefabcdefabcdefab.png", "leia-me.txt", usada}
	slices.Sort(esperado)
	if !slices.Equal(sobraram, esperado) {
		t.Fatalf("sobraram %v, esperava %v", sobraram, esperado)
	}

	// Trocada a imagem do cartão, a antiga fica sem uso e sai na próxima limpeza.
	e.app.db.Exec(`UPDATE notas SET imagem = '' WHERE id = ?`, notas[0].ID)
	if n, _ := e.app.limparMidia(context.Background(), time.Now().Add(-carenciaDaMidia)); n != 1 {
		t.Fatalf("a imagem que saiu do cartão devia ser apagada: %d", n)
	}
}

func TestCartoesDeUmTextoDoAcervo(t *testing.T) {
	e := novoAmbiente(t)
	texto := "TEXTO DO ACERVO. " + strings.Repeat("A Revolução Francesa começou em 1789. ", 300)
	pedir := func() geracao {
		var g geracao
		if s := e.pedir("POST", "/api/textos", map[string]any{
			"titulo": "História › Revolução Francesa", "baralho": "História", "contexto": "Tema História, capítulo Revolução Francesa.",
			"texto": texto, "link": "https://acervo/#/ler/historia/indice.md",
		}, &g); s != http.StatusCreated {
			t.Fatalf("pedir do texto: %d", s)
		}
		return g
	}
	g := pedir()
	if g.Origem != origemAcervo || g.Quantidade != 22 || g.Link == "" || g.BaralhoID != "" {
		t.Fatalf("pedido: %+v", g)
	}
	g = e.esperarGeracao(g.ID, emRevisao)
	obs := g.Proposta.Observacoes
	// O texto vai pela entrada, e a IA não usa a internet.
	if !strings.Contains(obs, "Recebeu o texto pela entrada.") || !strings.Contains(obs, "Seguiu a instrução do texto.") ||
		!strings.Contains(obs, "Ferramentas: ;") || !strings.Contains(obs, "WebSearch,WebFetch.") {
		t.Fatalf("observações: %q", obs)
	}
	// O baralho novo tem o nome do Tema, seja qual for o que a IA sugeriu.
	if g.Proposta.NomeBaralho != "História" {
		t.Fatalf("nome: %q", g.Proposta.NomeBaralho)
	}
	var r map[string]any
	if s := e.pedir("POST", "/api/geracoes/"+strconv.FormatInt(g.ID, 10)+"/aprovar", map[string]any{"nome_baralho": "História", "cartoes": g.Proposta.Cartoes[:1]}, &r); s != http.StatusOK {
		t.Fatalf("aprovar: %d", s)
	}

	// O próximo capítulo do mesmo Tema vai para o baralho que já existe e sabe dos cartões dele.
	g2 := pedir()
	if g2.BaralhoID != r["baralho_id"] {
		t.Fatalf("baralho do segundo: %q, esperava %v", g2.BaralhoID, r["baralho_id"])
	}
	g2 = e.esperarGeracao(g2.ID, emRevisao)
	if !strings.Contains(g2.Proposta.Observacoes, "Recebeu os cartões existentes.") || !g2.Proposta.Cartoes[0].Atualiza {
		t.Fatalf("segundo: %+v", g2.Proposta)
	}
	// Refazer usa o mesmo texto.
	if s := e.pedir("POST", "/api/geracoes/"+strconv.FormatInt(g2.ID, 10)+"/refazer", nil, nil); s != http.StatusOK {
		t.Fatalf("refazer: %d", s)
	}
	g2 = e.esperarGeracao(g2.ID, emRevisao)
	if !strings.Contains(g2.Proposta.Observacoes, "Recebeu o texto pela entrada.") {
		t.Fatalf("refeito: %q", g2.Proposta.Observacoes)
	}

	if s := e.pedir("POST", "/api/textos", map[string]any{"titulo": "x", "texto": "curto"}, nil); s != http.StatusBadRequest {
		t.Errorf("texto curto: %d", s)
	}
	if s := e.pedir("POST", "/api/textos", map[string]any{"titulo": "x", "texto": strings.Repeat("a", tamanhoMaximoDoTexto+1)}, nil); s != http.StatusRequestEntityTooLarge {
		t.Errorf("texto grande: %d", s)
	}
}
