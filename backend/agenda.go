package main

import (
	"time"

	"github.com/open-spaced-repetition/go-fsrs/v4"
)

// O dia de estudo vira às 4h, como no Anki: quem estuda depois da meia-noite ainda está no "hoje".
const viradaDoDia = 4 * time.Hour

type relogio struct {
	fuso  *time.Location
	agora func() time.Time
}

func novoRelogio(fuso *time.Location, agora func() time.Time) relogio {
	return relogio{fuso: fuso, agora: func() time.Time { return agora().UTC().Truncate(time.Second) }}
}

// dia devolve o começo e o fim do dia de estudo que contém t.
func (r relogio) dia(t time.Time) (inicio, fim time.Time) {
	l := t.In(r.fuso).Add(-viradaDoDia)
	inicio = time.Date(l.Year(), l.Month(), l.Day(), 4, 0, 0, 0, r.fuso)
	fim = time.Date(l.Year(), l.Month(), l.Day()+1, 4, 0, 0, 0, r.fuso)
	return inicio.UTC(), fim.UTC()
}

// O go-fsrs conta os dias de intervalo pela meia-noite UTC, que em Brasília é 21h: uma revisão às
// 22h e outra às 8h do dia seguinte contariam como o mesmo dia, e a segunda usaria a fórmula de
// curto prazo. Os horários entram no FSRS deslocados para a meia-noite UTC cair na virada do nosso
// dia (4h locais) e voltam ao normal na saída. O deslocamento é o mesmo nas duas pontas de uma
// resposta, então intervalos e vencimentos não mudam; só a contagem de dias passa a ser a nossa.
func (r relogio) deslocamento(t time.Time) time.Duration {
	_, seg := t.In(r.fuso).Zone()
	return time.Duration(seg)*time.Second - viradaDoDia
}

func mover(t time.Time, d time.Duration) time.Time {
	if t.IsZero() {
		return t
	}
	return t.Add(d).UTC()
}

// agendador: FSRS-6 com os pesos padrão. A retenção desejada é a do baralho (0,9 = acertar 90%
// das revisões). O fuzz espalha os vencimentos para os cartões criados juntos não vencerem juntos.
func agendador(retencao float64) *fsrs.FSRS {
	p := fsrs.DefaultParam()
	p.RequestRetention = retencao
	p.EnableFuzz = true
	return fsrs.NewFSRS(p)
}

func (c cartao) paraFSRS(d time.Duration) fsrs.Card {
	return fsrs.Card{
		Due:            mover(c.VenceEm, d),
		Stability:      c.Estabilidade,
		Difficulty:     c.Dificuldade,
		ScheduledDays:  uint64(c.DiasAgendados),
		Reps:           uint64(c.Repeticoes),
		Lapses:         uint64(c.Lapsos),
		State:          fsrs.State(c.Estado),
		LastReview:     mover(c.UltimaRevisao, d),
		RemainingSteps: c.PassosRestantes,
	}
}

func (c *cartao) aplicar(f fsrs.Card, d time.Duration) {
	c.VenceEm = mover(f.Due, -d).Truncate(time.Second)
	c.Estabilidade = f.Stability
	c.Dificuldade = f.Difficulty
	c.DiasAgendados = int64(f.ScheduledDays)
	c.Repeticoes = int64(f.Reps)
	c.Lapsos = int64(f.Lapses)
	c.Estado = int(f.State)
	c.UltimaRevisao = mover(f.LastReview, -d).Truncate(time.Second)
	c.PassosRestantes = f.RemainingSteps
}

// responderCartao aplica a nota (1 errei, 2 difícil, 3 bom, 4 fácil) e devolve o cartão agendado.
func (r relogio) responderCartao(c cartao, retencao float64, nota fsrs.Rating, agora time.Time) (cartao, error) {
	d := r.deslocamento(agora)
	info, err := agendador(retencao).Next(c.paraFSRS(d), mover(agora, d), nota)
	if err != nil {
		return c, err
	}
	c.aplicar(info.Card, d)
	return c, nil
}

// previsoes: em quantos segundos o cartão volta para cada nota, para mostrar nos botões.
func (r relogio) previsoes(c cartao, retencao float64, agora time.Time) (map[int]int64, error) {
	d := r.deslocamento(agora)
	log, err := agendador(retencao).Repeat(c.paraFSRS(d), mover(agora, d))
	if err != nil {
		return nil, err
	}
	saida := make(map[int]int64, 4)
	for _, n := range []fsrs.Rating{fsrs.Again, fsrs.Hard, fsrs.Good, fsrs.Easy} {
		saida[int(n)] = int64(mover(log[n].Card.Due, -d).Sub(agora).Round(time.Second) / time.Second)
	}
	return saida, nil
}
