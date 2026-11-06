package main

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
	_ "time/tzdata" // a imagem final não tem o banco de fusos; o dia de estudo depende dele

	_ "modernc.org/sqlite"
)

var version = "dev"

//go:embed all:web/dist
var webDist embed.FS

func main() {
	dataDir := env("DATA_DIR", "./data")
	addr := env("ADDR", ":8080")

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatal(err)
	}

	db, err := abrirBanco(filepath.Join(dataDir, "flashcards.db"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	fuso, err := time.LoadLocation(env("FUSO", "America/Sao_Paulo"))
	if err != nil {
		log.Fatal(err)
	}
	a := novoApp(db, novoRelogio(fuso, time.Now))

	web, err := fs.Sub(webDist, "web/dist")
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := &http.Server{
		Addr:              addr,
		Handler:           newServer(a, web, version),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("flashcards %s em %s", version, addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdown); err != nil {
		log.Print(err)
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func abrirBanco(caminho string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", caminho+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(esquema); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// Horários em RFC 3339, UTC e sem fração de segundo: assim a ordem do texto é a ordem do tempo.
const esquema = `
CREATE TABLE IF NOT EXISTS baralhos (
	id               TEXT PRIMARY KEY,
	nome             TEXT NOT NULL,
	novos_por_dia    INTEGER NOT NULL DEFAULT 20,
	revisoes_por_dia INTEGER NOT NULL DEFAULT 200,
	retencao         REAL NOT NULL DEFAULT 0.9,
	criado_em        TEXT NOT NULL,
	atualizado_em    TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS notas (
	id            TEXT PRIMARY KEY,
	baralho_id    TEXT NOT NULL REFERENCES baralhos (id) ON DELETE CASCADE,
	tipo          TEXT NOT NULL,
	frente        TEXT NOT NULL,
	verso         TEXT NOT NULL,
	tags          TEXT NOT NULL DEFAULT '',
	criada_em     TEXT NOT NULL,
	atualizada_em TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS notas_baralho ON notas (baralho_id);
CREATE TABLE IF NOT EXISTS cartoes (
	id               TEXT PRIMARY KEY,
	nota_id          TEXT NOT NULL REFERENCES notas (id) ON DELETE CASCADE,
	baralho_id       TEXT NOT NULL REFERENCES baralhos (id) ON DELETE CASCADE,
	sentido          INTEGER NOT NULL DEFAULT 0,
	posicao          INTEGER NOT NULL,
	estado           INTEGER NOT NULL DEFAULT 0,
	vence_em         TEXT NOT NULL,
	estabilidade     REAL NOT NULL DEFAULT 0,
	dificuldade      REAL NOT NULL DEFAULT 0,
	dias_agendados   INTEGER NOT NULL DEFAULT 0,
	repeticoes       INTEGER NOT NULL DEFAULT 0,
	lapsos           INTEGER NOT NULL DEFAULT 0,
	passos_restantes INTEGER NOT NULL DEFAULT 0,
	ultima_revisao   TEXT NOT NULL DEFAULT '',
	suspenso         INTEGER NOT NULL DEFAULT 0,
	UNIQUE (nota_id, sentido)
);
CREATE INDEX IF NOT EXISTS cartoes_fila ON cartoes (baralho_id, suspenso, estado, vence_em);
CREATE TABLE IF NOT EXISTS revisoes (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	cartao_id    TEXT NOT NULL REFERENCES cartoes (id) ON DELETE CASCADE,
	baralho_id   TEXT NOT NULL,
	nota         INTEGER NOT NULL,
	estado_antes INTEGER NOT NULL,
	revisado_em  TEXT NOT NULL,
	duracao_ms   INTEGER NOT NULL DEFAULT 0,
	antes        TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS revisoes_dia ON revisoes (baralho_id, revisado_em);
CREATE TABLE IF NOT EXISTS sugestoes (
	id            TEXT PRIMARY KEY,
	texto         TEXT NOT NULL,
	tela          TEXT NOT NULL DEFAULT '',
	estado        TEXT NOT NULL DEFAULT 'nova',
	resposta      TEXT NOT NULL DEFAULT '',
	criada_em     TEXT NOT NULL,
	atualizada_em TEXT NOT NULL
);
`
