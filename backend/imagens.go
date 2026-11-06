package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
)

// Imagens (e áudios) dos cartões. Ficam em DATA_DIR/media, com o nome tirado do conteúdo (o mesmo arquivo
// nunca é guardado duas vezes); o banco guarda só o nome. Vêm de dois lugares:
//   - da Wikipédia, pela API dela: a busca devolve a foto principal de cada artigo. É assim que a
//     IA "acha" imagens: ela indica o artigo, e o servidor busca a foto, sem a IA navegar;
//   - do aparelho, enviadas pelo editor.

const tamanhoMaximoDeImagem = 8 << 20

// O Wikimedia pede um User-Agent que identifique quem chama.
const agenteWiki = "Flashcards-homelab/1.0 (app pessoal; https://github.com/Lucas-Wesley/flashcards)"

var (
	nomeDeImagem = regexp.MustCompile(`^[0-9a-f]{32}\.(jpg|png|webp|gif)$`)
	nomeDeAudio  = regexp.MustCompile(`^[0-9a-f]{32}\.mp3$`)
)

// SVG fica de fora: pode carregar script. As miniaturas da Wikipédia de um SVG já vêm em PNG.
var extensoes = map[string]string{"image/jpeg": "jpg", "image/png": "png", "image/webp": "webp", "image/gif": "gif"}

var clienteWeb = &http.Client{Timeout: 20 * time.Second}

type imagemAchada struct {
	Titulo string `json:"titulo"`
	URL    string `json:"url"`
}

// buscarImagens procura artigos na Wikipédia em português e, sem resultado com foto, em inglês.
func (a *app) buscarImagens(ctx context.Context, termo string, limite int) ([]imagemAchada, error) {
	var ultimoErro error
	for _, idioma := range []string{"pt", "en"} {
		achadas, err := a.buscarNaWikipedia(ctx, idioma, termo, limite)
		if err != nil {
			ultimoErro = err
			continue
		}
		if len(achadas) > 0 {
			return achadas, nil
		}
	}
	return []imagemAchada{}, ultimoErro
}

func (a *app) buscarNaWikipedia(ctx context.Context, idioma, termo string, limite int) ([]imagemAchada, error) {
	base := a.cfg.wiki
	if strings.Contains(base, "%s") {
		base = fmt.Sprintf(base, idioma)
	}
	q := url.Values{
		"action": {"query"}, "format": {"json"}, "formatversion": {"2"},
		"generator": {"search"}, "gsrsearch": {termo}, "gsrnamespace": {"0"}, "gsrlimit": {"10"},
		"prop": {"pageimages"}, "piprop": {"thumbnail"}, "pithumbsize": {"800"},
	}
	req, err := http.NewRequestWithContext(ctx, "GET", base+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", agenteWiki)
	res, err := clienteWeb.Do(req)
	if err != nil {
		return nil, fmt.Errorf("a Wikipédia não respondeu: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("a Wikipédia respondeu %d", res.StatusCode)
	}
	var corpo struct {
		Query struct {
			Pages []struct {
				Title     string `json:"title"`
				Index     int    `json:"index"`
				Thumbnail struct {
					Source string `json:"source"`
				} `json:"thumbnail"`
			} `json:"pages"`
		} `json:"query"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 2<<20)).Decode(&corpo); err != nil {
		return nil, err
	}
	paginas := corpo.Query.Pages
	sort.Slice(paginas, func(i, j int) bool { return paginas[i].Index < paginas[j].Index })
	achadas := []imagemAchada{}
	for _, p := range paginas {
		if p.Thumbnail.Source != "" && len(achadas) < limite {
			achadas = append(achadas, imagemAchada{Titulo: p.Title, URL: p.Thumbnail.Source})
		}
	}
	return achadas, nil
}

// baixarImagem traz uma imagem de um servidor permitido (a Wikipédia) e devolve o nome guardado.
func (a *app) baixarImagem(ctx context.Context, endereco string) (string, error) {
	r, err := baixar(ctx, endereco, a.cfg.hostsDeImagem, "só dá para baixar imagens da Wikipédia")
	if err != nil {
		return "", err
	}
	defer r.Close()
	return a.guardarImagem(r)
}

// baixarAudio traz a pronúncia de uma frase, só dos sites que o app sabe ler (o do Mairo Vergara).
func (a *app) baixarAudio(ctx context.Context, endereco string) (string, error) {
	r, err := baixar(ctx, endereco, a.cfg.sitesMairo, "só dá para baixar áudios dos sites conhecidos")
	if err != nil {
		return "", err
	}
	defer r.Close()
	dados, err := lerAte(r, "áudio")
	if err != nil {
		return "", err
	}
	// MP3 com etiqueta ID3, ou começando direto num quadro de áudio (11 bits ligados).
	if http.DetectContentType(dados) != "audio/mpeg" && !(len(dados) > 1 && dados[0] == 0xFF && dados[1]&0xE0 == 0xE0) {
		return "", errors.New("o áudio não é um MP3")
	}
	return a.guardarMidia(dados, "mp3")
}

func baixar(ctx context.Context, endereco string, hosts []string, recusa string) (io.ReadCloser, error) {
	u, err := url.Parse(endereco)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || !slices.Contains(hosts, u.Host) {
		return nil, errors.New(recusa)
	}
	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", agenteWiki)
	res, err := clienteWeb.Do(req)
	if err != nil {
		return nil, fmt.Errorf("o arquivo não veio: %w", err)
	}
	if res.StatusCode != http.StatusOK {
		res.Body.Close()
		return nil, fmt.Errorf("o arquivo não veio: %d", res.StatusCode)
	}
	return res.Body, nil
}

func lerAte(r io.Reader, oQue string) ([]byte, error) {
	dados, err := io.ReadAll(io.LimitReader(r, tamanhoMaximoDeImagem+1))
	if err != nil {
		return nil, err
	}
	if len(dados) > tamanhoMaximoDeImagem {
		return nil, fmt.Errorf("%s maior que 8 MB", oQue)
	}
	return dados, nil
}

// guardarImagem confere o tipo pelo conteúdo (não pelo nome nem pelo cabeçalho) e grava.
func (a *app) guardarImagem(r io.Reader) (string, error) {
	dados, err := lerAte(r, "imagem")
	if err != nil {
		return "", err
	}
	ext, ok := extensoes[http.DetectContentType(dados)]
	if !ok {
		return "", errors.New("formato de imagem não aceito: use JPG, PNG, WebP ou GIF")
	}
	return a.guardarMidia(dados, ext)
}

func (a *app) guardarMidia(dados []byte, ext string) (string, error) {
	soma := sha256.Sum256(dados)
	nome := hex.EncodeToString(soma[:16]) + "." + ext
	destino := filepath.Join(a.cfg.dados, "media", nome)
	if _, err := os.Stat(destino); err == nil {
		return nome, nil
	}
	if err := os.WriteFile(destino+".tmp", dados, 0o644); err != nil {
		return "", err
	}
	return nome, os.Rename(destino+".tmp", destino)
}

// ---------- Handlers ----------

func (a *app) handleBuscarImagens(w http.ResponseWriter, r *http.Request) {
	termo := strings.TrimSpace(r.URL.Query().Get("q"))
	if termo == "" {
		erroJSON(w, http.StatusBadRequest, "Diga o que procurar.")
		return
	}
	achadas, err := a.buscarImagens(r.Context(), termo, 8)
	if err != nil && len(achadas) == 0 {
		erroJSON(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, achadas)
}

func (a *app) handleBaixarImagem(w http.ResponseWriter, r *http.Request) {
	var corpo struct {
		URL string `json:"url"`
	}
	if !lerCorpo(w, r, &corpo) {
		return
	}
	nome, err := a.baixarImagem(r.Context(), corpo.URL)
	if err != nil {
		erroJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"nome": nome})
}

func (a *app) handleEnviarImagem(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, tamanhoMaximoDeImagem+1<<20)
	arquivo, _, err := r.FormFile("arquivo")
	if err != nil {
		erroJSON(w, http.StatusBadRequest, "Envie uma imagem de até 8 MB.")
		return
	}
	defer arquivo.Close()
	nome, err := a.guardarImagem(arquivo)
	if err != nil {
		erroJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"nome": nome})
}

func (a *app) handleImagem(w http.ResponseWriter, r *http.Request) {
	a.servirMidia(w, r, nomeDeImagem)
}

func (a *app) handleAudio(w http.ResponseWriter, r *http.Request) {
	a.servirMidia(w, r, nomeDeAudio)
}

func (a *app) servirMidia(w http.ResponseWriter, r *http.Request, nomes *regexp.Regexp) {
	nome := r.PathValue("nome")
	if !nomes.MatchString(nome) {
		erroJSON(w, http.StatusNotFound, "Arquivo não encontrado.")
		return
	}
	// O nome é o conteúdo: a mesma URL nunca muda de imagem.
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeFile(w, r, filepath.Join(a.cfg.dados, "media", nome))
}

// ---------- Limpeza ----------

// Uma mídia que nenhum cartão usa (a imagem trocada no editor, a de um cartão apagado, a enviada e
// abandonada) só ocupa espaço. A limpeza roda ao subir o app e uma vez por dia; o arquivo só sai
// depois de um dia sem uso, para não levar o que o editor acabou de enviar e ainda não salvou.
const (
	intervaloDaLimpeza = 24 * time.Hour
	carenciaDaMidia    = 24 * time.Hour
)

func (a *app) limparMidiaSempre(ctx context.Context) {
	for {
		if n, err := a.limparMidia(ctx, time.Now().Add(-carenciaDaMidia)); err != nil {
			log.Printf("limpeza da mídia: %v", err)
		} else if n > 0 {
			log.Printf("limpeza da mídia: %d arquivos sem uso apagados", n)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(intervaloDaLimpeza):
		}
	}
}

// limparMidia apaga os arquivos de mídia sem uso modificados antes de antesDe. Conta como em uso o
// que uma nota guarda e o que uma proposta em revisão aponta (o cartão que ela vai atualizar).
func (a *app) limparMidia(ctx context.Context, antesDe time.Time) (int, error) {
	usados := map[string]bool{}
	rows, err := a.db.QueryContext(ctx, `SELECT imagem, audio FROM notas WHERE imagem != '' OR audio != ''`)
	if err != nil {
		return 0, err
	}
	for rows.Next() {
		var imagem, audio string
		if err := rows.Scan(&imagem, &audio); err != nil {
			rows.Close()
			return 0, err
		}
		usados[imagem], usados[audio] = true, true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	rows, err = a.db.QueryContext(ctx, `SELECT `+colunasGeracao+` FROM geracoes WHERE estado = ?`, emRevisao)
	if err != nil {
		return 0, err
	}
	for rows.Next() {
		g, err := lerGeracao(rows)
		if err != nil {
			rows.Close()
			return 0, err
		}
		for _, c := range g.Proposta.cartoesOuNada() {
			usados[strings.TrimPrefix(c.ImagemURL, "/api/imagens/")] = true
			usados[strings.TrimPrefix(c.AudioURL, "/api/audios/")] = true
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	pasta := filepath.Join(a.cfg.dados, "media")
	arquivos, err := os.ReadDir(pasta)
	if err != nil {
		return 0, err
	}
	apagados := 0
	for _, arq := range arquivos {
		nome := arq.Name()
		// Só o que o app grava: mídia com nome de conteúdo e as sobras .tmp de uma gravação interrompida.
		ehMidia := nomeDeImagem.MatchString(nome) || nomeDeAudio.MatchString(nome)
		if arq.IsDir() || usados[nome] || !(ehMidia || strings.HasSuffix(nome, ".tmp")) {
			continue
		}
		info, err := arq.Info()
		if err != nil || !info.ModTime().Before(antesDe) {
			continue
		}
		if err := os.Remove(filepath.Join(pasta, nome)); err != nil {
			return apagados, err
		}
		apagados++
	}
	return apagados, nil
}

func (p *proposta) cartoesOuNada() []cartaoProposto {
	if p == nil {
		return nil
	}
	return p.Cartoes
}
