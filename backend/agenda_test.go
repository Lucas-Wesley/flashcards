package main

import (
	"math"
	"testing"
	"time"

	"github.com/open-spaced-repetition/go-fsrs/v4"
)

func relogioDeTeste(t *testing.T) (relogio, *time.Location) {
	t.Helper()
	fuso, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Fatal(err)
	}
	return novoRelogio(fuso, time.Now), fuso
}

func TestDiaDeEstudoViraAs4h(t *testing.T) {
	r, fuso := relogioDeTeste(t)
	casos := []struct {
		agora  time.Time
		inicio time.Time
	}{
		{time.Date(2026, 11, 7, 3, 59, 0, 0, fuso), time.Date(2026, 11, 6, 4, 0, 0, 0, fuso)},
		{time.Date(2026, 11, 7, 4, 0, 0, 0, fuso), time.Date(2026, 11, 7, 4, 0, 0, 0, fuso)},
		{time.Date(2026, 11, 6, 23, 30, 0, 0, fuso), time.Date(2026, 11, 6, 4, 0, 0, 0, fuso)},
	}
	for _, c := range casos {
		inicio, fim := r.dia(c.agora)
		if !inicio.Equal(c.inicio) || !fim.Equal(c.inicio.AddDate(0, 0, 1)) {
			t.Errorf("%v: dia de %v a %v, esperava começar em %v", c.agora, inicio.In(fuso), fim.In(fuso), c.inicio)
		}
	}
}

// Uma revisão às 22h e outra às 8h do dia seguinte são dias diferentes; às 3h ainda é o mesmo dia.
// Sem o deslocamento, o go-fsrs veria as 22h de Brasília (1h UTC) e as 8h no mesmo dia UTC.
func TestDiasDoFSRSContamPelaVirada(t *testing.T) {
	r, fuso := relogioDeTeste(t)
	ontem := time.Date(2026, 11, 6, 22, 0, 0, 0, fuso)
	c := cartao{Estado: revisao, Estabilidade: 10, Dificuldade: 5, DiasAgendados: 10, Repeticoes: 5,
		UltimaRevisao: ontem.UTC(), VenceEm: ontem.AddDate(0, 0, 10).UTC()}
	p := agendador(0.9).Parameters
	atual := &fsrs.MemoryState{Stability: 10, Difficulty: 5}

	casos := []struct {
		nome     string
		agora    time.Time
		passados uint64
	}{
		{"às 3h, ainda o mesmo dia de estudo", time.Date(2026, 11, 7, 3, 0, 0, 0, fuso), 0},
		{"às 8h, já o dia seguinte", time.Date(2026, 11, 7, 8, 0, 0, 0, fuso), 1},
	}
	for _, caso := range casos {
		depois, err := r.responderCartao(c, 0.9, fsrs.Good, caso.agora.UTC())
		if err != nil {
			t.Fatal(err)
		}
		esperado := p.NextState(atual, 0.9, caso.passados, fsrs.Good).Memory.Stability
		if math.Abs(depois.Estabilidade-esperado) > 1e-9 {
			t.Errorf("%s: estabilidade %v, esperava %v (%d dia(s) passados)", caso.nome, depois.Estabilidade, esperado, caso.passados)
		}
		if !depois.UltimaRevisao.Equal(caso.agora) {
			t.Errorf("%s: última revisão %v, esperava %v", caso.nome, depois.UltimaRevisao, caso.agora)
		}
	}
}

// O caminho de um cartão novo com os passos padrão (1 min e 10 min), como no Anki.
func TestCaminhoDeUmCartaoNovo(t *testing.T) {
	r, fuso := relogioDeTeste(t)
	agora := time.Date(2026, 11, 6, 10, 0, 0, 0, fuso).UTC()
	c := cartao{VenceEm: agora}

	prev, err := r.previsoes(c, 0.9, agora)
	if err != nil {
		t.Fatal(err)
	}
	if prev[1] != 60 || prev[2] != 360 || prev[3] != 600 {
		t.Errorf("novo: errei/difícil/bom em %ds/%ds/%ds, esperava 1, 6 e 10 min", prev[1], prev[2], prev[3])
	}
	// Fácil gradua direto: estabilidade inicial de 8,3 dias, com fuzz de até ±1,8 dia.
	if d := prev[4] / 86400; d < 6 || d > 10 || prev[4]%86400 != 0 {
		t.Errorf("novo: fácil em %ds, esperava de 6 a 10 dias", prev[4])
	}

	c, _ = r.responderCartao(c, 0.9, fsrs.Good, agora)
	if c.Estado != aprendendo || !c.VenceEm.Equal(agora.Add(10*time.Minute)) {
		t.Fatalf("bom no novo: estado %d, vence %v", c.Estado, c.VenceEm)
	}

	agora = agora.Add(10 * time.Minute)
	c, _ = r.responderCartao(c, 0.9, fsrs.Good, agora)
	if c.Estado != revisao || c.DiasAgendados != 2 || !c.VenceEm.Equal(agora.AddDate(0, 0, 2)) {
		t.Fatalf("bom no segundo passo: estado %d, %d dias, vence %v", c.Estado, c.DiasAgendados, c.VenceEm)
	}

	// Revisões com "bom" no dia certo: o intervalo cresce a cada vez.
	anterior := c.DiasAgendados
	for i := range 6 {
		agora = c.VenceEm
		prev, _ := r.previsoes(c, 0.9, agora)
		if !(prev[1] < prev[2] && prev[2] < prev[3] && prev[3] < prev[4]) {
			t.Fatalf("revisão %d: botões fora de ordem: %v", i, prev)
		}
		c, _ = r.responderCartao(c, 0.9, fsrs.Good, agora)
		if c.DiasAgendados <= anterior {
			t.Fatalf("revisão %d: intervalo %d não cresceu (antes %d)", i, c.DiasAgendados, anterior)
		}
		anterior = c.DiasAgendados
	}
	if anterior < 60 {
		t.Errorf("depois de 8 acertos o intervalo devia passar de 2 meses, ficou em %d dias", anterior)
	}

	// Errar uma revisão: volta a reaprender em 10 min e conta um lapso; a estabilidade cai.
	agora = c.VenceEm
	antes := c.Estabilidade
	c, _ = r.responderCartao(c, 0.9, fsrs.Again, agora)
	if c.Estado != reaprendendo || c.Lapsos != 1 || !c.VenceEm.Equal(agora.Add(10*time.Minute)) || c.Estabilidade >= antes {
		t.Fatalf("errei na revisão: estado %d, lapsos %d, vence %v, estabilidade %v → %v", c.Estado, c.Lapsos, c.VenceEm, antes, c.Estabilidade)
	}
	agora = c.VenceEm
	c, _ = r.responderCartao(c, 0.9, fsrs.Good, agora)
	if c.Estado != revisao || c.DiasAgendados < 1 || c.DiasAgendados >= anterior {
		t.Fatalf("depois de reaprender: estado %d, %d dias", c.Estado, c.DiasAgendados)
	}
}

// Retenção desejada maior pede revisões mais cedo.
func TestRetencaoMaiorEncurtaOIntervalo(t *testing.T) {
	r, fuso := relogioDeTeste(t)
	agora := time.Date(2026, 11, 6, 10, 0, 0, 0, fuso).UTC()
	c := cartao{Estado: revisao, Estabilidade: 30, Dificuldade: 5, DiasAgendados: 30, Repeticoes: 4,
		UltimaRevisao: agora.AddDate(0, 0, -30), VenceEm: agora}
	a, _ := r.responderCartao(c, 0.8, fsrs.Good, agora)
	b, _ := r.responderCartao(c, 0.95, fsrs.Good, agora)
	if b.DiasAgendados >= a.DiasAgendados {
		t.Errorf("retenção 95%%: %d dias; 80%%: %d dias", b.DiasAgendados, a.DiasAgendados)
	}
}

// "Amanhã" é um erro para a memória do cartão, mas sem os passos de minutos: volta em 24h.
func TestAmanhaEhErroSemPassos(t *testing.T) {
	r, fuso := relogioDeTeste(t)
	agora := time.Date(2026, 11, 6, 22, 0, 0, 0, fuso).UTC()

	novoCartao, err := r.responderCartao(cartao{VenceEm: agora}, 0.9, amanha, agora)
	if err != nil {
		t.Fatal(err)
	}
	errei, _ := r.responderCartao(cartao{VenceEm: agora}, 0.9, fsrs.Again, agora)
	if novoCartao.Estado != revisao || novoCartao.DiasAgendados != 1 || !novoCartao.VenceEm.Equal(agora.Add(24*time.Hour)) ||
		novoCartao.Estabilidade != errei.Estabilidade || novoCartao.Dificuldade != errei.Dificuldade {
		t.Fatalf("novo com amanhã: %+v (errei daria estabilidade %v)", novoCartao, errei.Estabilidade)
	}

	// Um cartão antigo e estável: o FSRS sugeriria dias depois do erro, mas volta amanhã, com lapso.
	estavel := cartao{Estado: revisao, Estabilidade: 200, Dificuldade: 4, DiasAgendados: 180, Repeticoes: 9,
		UltimaRevisao: agora.AddDate(0, 0, -180), VenceEm: agora}
	c, _ := r.responderCartao(estavel, 0.9, amanha, agora)
	errei, _ = r.responderCartao(estavel, 0.9, fsrs.Again, agora)
	if c.Estado != revisao || c.Lapsos != 1 || c.DiasAgendados != 1 || !c.VenceEm.Equal(agora.Add(24*time.Hour)) ||
		math.Abs(c.Estabilidade-errei.Estabilidade) > 1e-9 {
		t.Fatalf("revisão com amanhã: %+v (errei daria estabilidade %v)", c, errei.Estabilidade)
	}

	prev, _ := r.previsoes(estavel, 0.9, agora)
	if prev[int(amanha)] != 86400 {
		t.Errorf("previsão de amanhã: %d", prev[int(amanha)])
	}
}
