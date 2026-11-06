package main

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type config struct {
	dados  string // DATA_DIR
	regras string // instruções padrão da IA (regras/ do repositório)
	claude string
	modelo string // vazio = o padrão do Claude Code
	// Busca de imagens: a API da Wikipédia ("%s" é o idioma) e os servidores de onde o app aceita
	// baixar imagem. Os testes trocam os dois por um servidor falso.
	wiki          string
	hostsDeImagem []string
	// Sites de onde o app sabe tirar cartões de um link (o leitor do Mairo Vergara). O áudio das
	// frases também vem deles.
	sitesMairo []string
}

func configDoAmbiente(dados string) config {
	return config{
		dados:         absoluto(dados),
		regras:        absoluto(env("REGRAS_DIR", "../regras")),
		claude:        env("CLAUDE_BIN", "claude"),
		modelo:        os.Getenv("CLAUDE_MODEL"),
		wiki:          env("WIKI_API", "https://%s.wikipedia.org/w/api.php"),
		hostsDeImagem: strings.Split(env("HOSTS_DE_IMAGEM", "upload.wikimedia.org,thumb.wikimedia.org"), ","),
		sitesMairo:    strings.Split(env("SITES_MAIRO", "www.mairovergara.com,mairovergara.com"), ","),
	}
}

// absoluto: o claude roda em outra pasta, e um caminho relativo apontaria errado.
func absoluto(caminho string) string {
	if abs, err := filepath.Abs(caminho); err == nil {
		return abs
	}
	return caminho
}

// app junta o que os handlers e a fila usam.
type app struct {
	db   *sql.DB
	rel  relogio
	cfg  config
	fila *filaDaIA
}

func novoApp(db *sql.DB, rel relogio, cfg config) (*app, error) {
	for _, pasta := range []string{"media", "geracoes"} {
		if err := os.MkdirAll(filepath.Join(cfg.dados, pasta), 0o755); err != nil {
			return nil, err
		}
	}
	a := &app{db: db, rel: rel, cfg: cfg}
	a.fila = novaFila(a)
	return a, nil
}

// Estados do cartão: os mesmos números do FSRS.
const (
	novo         = 0
	aprendendo   = 1
	revisao      = 2
	reaprendendo = 3
)

type baralho struct {
	ID             string  `json:"id"`
	Nome           string  `json:"nome"`
	NovosPorDia    int     `json:"novos_por_dia"`
	RevisoesPorDia int     `json:"revisoes_por_dia"`
	Retencao       float64 `json:"retencao"`
	CriadoEm       string  `json:"criado_em"`
	AtualizadoEm   string  `json:"atualizado_em"`
}

type cartao struct {
	ID              string    `json:"id"`
	NotaID          string    `json:"nota_id"`
	BaralhoID       string    `json:"baralho_id"`
	Sentido         int       `json:"sentido"` // 0 frente → verso; 1 verso → frente
	Posicao         int64     `json:"posicao"`
	Estado          int       `json:"estado"`
	VenceEm         time.Time `json:"vence_em"`
	Estabilidade    float64   `json:"estabilidade"`
	Dificuldade     float64   `json:"dificuldade"`
	DiasAgendados   int64     `json:"dias_agendados"`
	Repeticoes      int64     `json:"repeticoes"`
	Lapsos          int64     `json:"lapsos"`
	PassosRestantes int       `json:"passos_restantes"`
	UltimaRevisao   time.Time `json:"ultima_revisao"`
	Suspenso        bool      `json:"suspenso"`
}

const colunasCartao = `id, nota_id, baralho_id, sentido, posicao, estado, vence_em, estabilidade, dificuldade,
	dias_agendados, repeticoes, lapsos, passos_restantes, ultima_revisao, suspenso`

type linha interface{ Scan(...any) error }

func lerCartao(l linha) (cartao, error) {
	var c cartao
	var vence, ultima string
	err := l.Scan(&c.ID, &c.NotaID, &c.BaralhoID, &c.Sentido, &c.Posicao, &c.Estado, &vence, &c.Estabilidade,
		&c.Dificuldade, &c.DiasAgendados, &c.Repeticoes, &c.Lapsos, &c.PassosRestantes, &ultima, &c.Suspenso)
	c.VenceEm = lerHora(vence)
	c.UltimaRevisao = lerHora(ultima)
	return c, err
}

// salvarAgenda grava o que o FSRS muda num cartão.
func salvarAgenda(ex executor, c cartao) error {
	_, err := ex.Exec(`UPDATE cartoes SET estado = ?, vence_em = ?, estabilidade = ?, dificuldade = ?,
		dias_agendados = ?, repeticoes = ?, lapsos = ?, passos_restantes = ?, ultima_revisao = ? WHERE id = ?`,
		c.Estado, hora(c.VenceEm), c.Estabilidade, c.Dificuldade, c.DiasAgendados, c.Repeticoes, c.Lapsos,
		c.PassosRestantes, hora(c.UltimaRevisao), c.ID)
	return err
}

type executor interface {
	Exec(string, ...any) (sql.Result, error)
}

func hora(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func lerHora(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

func (a *app) agora() string {
	return hora(a.rel.agora())
}

// novoID: UUID v4, como pede o contrato do ecossistema.
func novoID() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
