package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"unicode"
)

// Busca de imagens em fontes abertas, sem chave:
//   - Wikipédia (pt, e en sem resultado): a foto principal de cada artigo;
//   - Wikimedia Commons: os arquivos em si, o que acha o que não é a foto principal de um artigo
//     (a bandeira do Império, não só a atual);
//   - Openverse: fotos com licença aberta de outros acervos (Flickr, museus…), pela miniatura que o
//     próprio Openverse serve. Só na busca manual: a geração de 30 cartões estouraria o limite dele.
//
// O termo vai para as fontes; o contexto (frente, verso e tags do cartão) só ordena o que voltou.
// Assim, "Bandeira do Brasil" num cartão sobre 1822 põe a bandeira do Império antes da atual.

type imagemAchada struct {
	Titulo  string `json:"titulo"`
	URL     string `json:"url"`
	Fonte   string `json:"fonte"`
	Licenca string `json:"licenca,omitempty"`
	Autor   string `json:"autor,omitempty"`
	pontos  int
	ordem   int
}

type fonteDeImagem func(ctx context.Context, termo string) ([]imagemAchada, error)

// buscarImagens junta as fontes, ordena pelo termo e pelo contexto e devolve até limite imagens.
func (a *app) buscarImagens(ctx context.Context, termo, contexto string, limite int, comOpenverse bool) ([]imagemAchada, error) {
	palavrasDoTermo, palavrasDoContexto := palavras(termo), palavras(contexto)
	fontes := []fonteDeImagem{a.buscarNaWikipedia, a.buscarNoCommons}
	// O termo costuma ser o assunto ("Bandeira do Brasil"); o ano que o cartão cita leva à época certa.
	var anos []string
	for w := range palavrasDoContexto {
		if ano.MatchString(w) && !palavrasDoTermo[w] {
			anos = append(anos, w)
		}
	}
	if len(anos) > 0 {
		slices.Sort(anos)
		fontes = append(fontes, func(ctx context.Context, termo string) ([]imagemAchada, error) {
			return a.buscarNoCommons(ctx, termo+" "+anos[0])
		})
	}
	if comOpenverse && a.cfg.openverse != "" {
		fontes = append(fontes, a.buscarNoOpenverse)
	}
	resultados := make([][]imagemAchada, len(fontes))
	erros := make([]error, len(fontes))
	var wg sync.WaitGroup
	for i, f := range fontes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resultados[i], erros[i] = f(ctx, termo)
		}()
	}
	wg.Wait()

	vistas := map[string]bool{}
	achadas := []imagemAchada{}
	for _, lista := range resultados {
		for i, im := range lista {
			if vistas[im.URL] {
				continue
			}
			vistas[im.URL] = true
			im.ordem = i
			im.pontos = pontuar(im.Titulo, palavrasDoTermo, palavrasDoContexto)
			achadas = append(achadas, im)
		}
	}
	// Empate: a ordem das fontes (Wikipédia, Commons, Openverse) e a de cada uma.
	sort.SliceStable(achadas, func(i, j int) bool {
		if achadas[i].pontos != achadas[j].pontos {
			return achadas[i].pontos > achadas[j].pontos
		}
		return achadas[i].ordem < achadas[j].ordem
	})
	if len(achadas) > limite {
		achadas = achadas[:limite]
	}
	if len(achadas) == 0 {
		for _, err := range erros {
			if err != nil {
				return achadas, err
			}
		}
	}
	return achadas, nil
}

var ano = regexp.MustCompile(`^(1[0-9]|20)[0-9]{2}$`)

// pontuar: palavras do título que estão no termo valem mais que as que só estão no contexto; um ano
// do título que o cartão cita vale muito, e um que ele não cita conta contra (outra época), como cada
// palavra que não está nem no termo nem no cartão. O título
// igual ao termo não ganha nada a mais: "Bandeira do Brasil" é o assunto, não a resposta.
func pontuar(titulo string, doTermo, doContexto map[string]bool) int {
	p, assunto := 0, false
	for w := range palavras(titulo) {
		switch {
		case ano.MatchString(w) && (doContexto[w] || doTermo[w]):
			p += 8
		case ano.MatchString(w):
			p -= 2
		case doTermo[w]:
			p += 3
			assunto = true
		case doContexto[w]:
			p += 2
			assunto = true
		default:
			p-- // o título fala de outra coisa ("da cidade do Rio de Janeiro")
		}
	}
	// Só o ano não basta: o artigo "1889" não é a bandeira de 1889.
	if !assunto {
		return min(p, 0)
	}
	return p
}

var palavrasVazias = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`que qual quais quem quando onde como por porque para com sem uma uns umas dos das nos nas pelo pela
		pelos pelas foi era sao ser seu sua seus suas este esta esse essa isso isto aquele mais menos muito entre sobre ate
		desde apos the and for with from of file arquivo svg png jpg jpeg gif webp`) {
		palavrasVazias[w] = true
	}
}

// Os arquivos do Commons costumam ter título em inglês: as palavras mais comuns viram a de português,
// para "Flag of Brazil (1822–1853)" combinar com um cartão sobre a bandeira do Brasil em 1822.
var doIngles = map[string]string{
	"flag": "bandeira", "flags": "bandeira", "brazil": "brasil", "empire": "imperio", "imperial": "imperio",
	"kingdom": "reino", "republic": "republica", "state": "estado", "city": "cidade", "map": "mapa",
	"portrait": "retrato", "president": "presidente", "coat": "brasao", "arms": "brasao", "church": "igreja",
	"cathedral": "catedral", "river": "rio", "king": "rei", "queen": "rainha", "emperor": "imperador",
	"painting": "pintura", "statue": "estatua", "palace": "palacio", "war": "guerra", "battle": "batalha",
}

// palavras: minúsculas, sem acento, sem as vazias e sem as de duas letras.
func palavras(texto string) map[string]bool {
	ws := map[string]bool{}
	for _, w := range strings.FieldsFunc(semAcento(strings.ToLower(texto)), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if len(w) > 2 && !palavrasVazias[w] {
			if pt, ok := doIngles[w]; ok {
				w = pt
			}
			ws[w] = true
		}
	}
	return ws
}

var acentos = strings.NewReplacer("á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a", "é", "e", "ê", "e", "è", "e", "í", "i", "î", "i",
	"ó", "o", "ô", "o", "õ", "o", "ö", "o", "ú", "u", "ü", "u", "ç", "c", "ñ", "n")

func semAcento(s string) string { return acentos.Replace(s) }

// ---------- Fontes ----------

func (a *app) consultar(ctx context.Context, endereco, quem string, destino any) error {
	req, err := http.NewRequestWithContext(ctx, "GET", endereco, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", agenteWiki)
	res, err := clienteWeb.Do(req)
	if err != nil {
		return fmt.Errorf("%s não respondeu: %w", quem, err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%s respondeu %d", quem, res.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(destino)
}

// buscarNaWikipedia: em português e, sem nenhum artigo com foto, em inglês.
func (a *app) buscarNaWikipedia(ctx context.Context, termo string) ([]imagemAchada, error) {
	var ultimoErro error
	for _, idioma := range []string{"pt", "en"} {
		base := a.cfg.wiki
		if strings.Contains(base, "%s") {
			base = fmt.Sprintf(base, idioma)
		}
		q := url.Values{
			"action": {"query"}, "format": {"json"}, "formatversion": {"2"},
			"generator": {"search"}, "gsrsearch": {termo}, "gsrnamespace": {"0"}, "gsrlimit": {"10"},
			"prop": {"pageimages"}, "piprop": {"thumbnail"}, "pithumbsize": {"800"},
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
		if err := a.consultar(ctx, base+"?"+q.Encode(), "a Wikipédia", &corpo); err != nil {
			ultimoErro = err
			continue
		}
		paginas := corpo.Query.Pages
		sort.Slice(paginas, func(i, j int) bool { return paginas[i].Index < paginas[j].Index })
		achadas := []imagemAchada{}
		for _, p := range paginas {
			if p.Thumbnail.Source != "" {
				achadas = append(achadas, imagemAchada{Titulo: p.Title, URL: p.Thumbnail.Source, Fonte: "Wikipédia"})
			}
		}
		if len(achadas) > 0 {
			return achadas, nil
		}
	}
	return nil, ultimoErro
}

var etiquetas = regexp.MustCompile(`<[^>]*>`)

func (a *app) buscarNoCommons(ctx context.Context, termo string) ([]imagemAchada, error) {
	if a.cfg.commons == "" {
		return nil, nil
	}
	q := url.Values{
		"action": {"query"}, "format": {"json"}, "formatversion": {"2"},
		"generator": {"search"}, "gsrsearch": {termo + " filetype:bitmap|drawing"}, "gsrnamespace": {"6"}, "gsrlimit": {"12"},
		"prop": {"imageinfo"}, "iiprop": {"url|mime|extmetadata"}, "iiurlwidth": {"800"},
		"iiextmetadatafilter": {"LicenseShortName|Artist"},
	}
	var corpo struct {
		Query struct {
			Pages []struct {
				Title     string `json:"title"`
				Index     int    `json:"index"`
				ImageInfo []struct {
					ThumbURL    string `json:"thumburl"`
					Mime        string `json:"mime"`
					ExtMetadata map[string]struct {
						Value string `json:"value"`
					} `json:"extmetadata"`
				} `json:"imageinfo"`
			} `json:"pages"`
		} `json:"query"`
	}
	if err := a.consultar(ctx, a.cfg.commons+"?"+q.Encode(), "o Wikimedia Commons", &corpo); err != nil {
		return nil, err
	}
	paginas := corpo.Query.Pages
	sort.Slice(paginas, func(i, j int) bool { return paginas[i].Index < paginas[j].Index })
	achadas := []imagemAchada{}
	for _, p := range paginas {
		if len(p.ImageInfo) == 0 || p.ImageInfo[0].ThumbURL == "" || !strings.HasPrefix(p.ImageInfo[0].Mime, "image/") {
			continue
		}
		ii := p.ImageInfo[0]
		titulo := p.Title
		if _, depois, ok := strings.Cut(titulo, ":"); ok {
			titulo = depois
		}
		if i := strings.LastIndex(titulo, "."); i > 0 {
			titulo = titulo[:i]
		}
		autor := strings.TrimSpace(etiquetas.ReplaceAllString(ii.ExtMetadata["Artist"].Value, ""))
		achadas = append(achadas, imagemAchada{Titulo: titulo, URL: ii.ThumbURL, Fonte: "Wikimedia Commons",
			Licenca: ii.ExtMetadata["LicenseShortName"].Value, Autor: cortar(autor, 80)})
	}
	return achadas, nil
}

func (a *app) buscarNoOpenverse(ctx context.Context, termo string) ([]imagemAchada, error) {
	// O Commons já entra pela própria API; do Openverse, só os outros acervos.
	q := url.Values{"q": {termo}, "page_size": {"12"}, "excluded_source": {"wikimedia"}, "mature": {"false"}}
	var corpo struct {
		Results []struct {
			Title          string `json:"title"`
			Thumbnail      string `json:"thumbnail"`
			License        string `json:"license"`
			LicenseVersion string `json:"license_version"`
			Creator        string `json:"creator"`
			Source         string `json:"source"`
		} `json:"results"`
	}
	if err := a.consultar(ctx, a.cfg.openverse+"?"+q.Encode(), "o Openverse", &corpo); err != nil {
		return nil, err
	}
	achadas := []imagemAchada{}
	for _, r := range corpo.Results {
		if r.Thumbnail == "" {
			continue
		}
		licenca := strings.ToUpper(r.License)
		switch licenca {
		case "PDM":
			licenca = "Domínio público"
		case "CC0":
		default:
			licenca = strings.TrimSpace("CC " + licenca + " " + r.LicenseVersion)
		}
		achadas = append(achadas, imagemAchada{Titulo: strings.TrimSpace(r.Title), URL: r.Thumbnail, Fonte: "Openverse · " + r.Source,
			Licenca: licenca, Autor: cortar(r.Creator, 80)})
	}
	return achadas, nil
}
