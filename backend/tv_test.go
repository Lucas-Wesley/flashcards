package main

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// A TV (F37) recebe a cena atual ao abrir e cada mudança que o celular manda; a porta dela tem só a
// página, a cena e as imagens.
func TestEstudarNaTV(t *testing.T) {
	e := novoAmbiente(t)
	e.app.cfg.enderecoDaTV = "http://192.168.1.20:8453"
	tv := httptest.NewServer(servidorDaTV(e.app))
	defer tv.Close()

	var info struct {
		Endereco string `json:"endereco"`
		Telas    int    `json:"telas"`
	}
	e.pedir("GET", "/api/tv", nil, &info)
	if info.Endereco != "http://192.168.1.20:8453" || info.Telas != 0 {
		t.Errorf("GET /api/tv: %+v", info)
	}
	if s := e.pedir("PUT", "/api/tv", map[string]any{"frente": "sem ativo"}, nil); s != http.StatusBadRequest {
		t.Errorf("cena sem ativo: %d", s)
	}

	res, err := http.Get(tv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	pagina, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if !strings.Contains(string(pagina), "Flashcards na TV") || !strings.Contains(string(pagina), "new EventSource('/cena')") {
		t.Errorf("página da TV:\n%.300s", pagina)
	}
	if res, _ := http.Get(tv.URL + "/api/baralhos"); res.StatusCode != http.StatusNotFound {
		t.Errorf("a porta da TV abriu a API: %d", res.StatusCode)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", tv.URL+"/cena", nil)
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	linhas := bufio.NewScanner(res.Body)
	proxima := func() map[string]any {
		t.Helper()
		for linhas.Scan() {
			if d, ok := strings.CutPrefix(linhas.Text(), "data: "); ok {
				var cena map[string]any
				if err := json.Unmarshal([]byte(d), &cena); err != nil {
					t.Fatalf("cena ilegível: %q", d)
				}
				return cena
			}
		}
		t.Fatal("a cena não veio")
		return nil
	}

	if c := proxima(); c["ativo"] != false {
		t.Errorf("ao abrir, a TV espera: %v", c)
	}
	e.pedir("GET", "/api/tv", nil, &info)
	if info.Telas != 1 {
		t.Errorf("TV aberta não contou: %+v", info)
	}
	// Mesmo com a cena em várias linhas, ela chega inteira.
	req2, _ := http.NewRequest("PUT", e.srv.URL+"/api/tv", strings.NewReader("{\n  \"ativo\": true,\n  \"frente\": \"Capital da França?\\nem uma palavra\",\n  \"mostrando\": false\n}"))
	if res, err := http.DefaultClient.Do(req2); err != nil || res.StatusCode != http.StatusNoContent {
		t.Fatalf("PUT cena: %v %v", err, res)
	}
	if c := proxima(); c["ativo"] != true || c["frente"] != "Capital da França?\nem uma palavra" {
		t.Errorf("cena do cartão: %v", c)
	}
	e.pedir("PUT", "/api/tv", map[string]any{"ativo": false}, nil)
	if c := proxima(); c["ativo"] != false {
		t.Errorf("fora do estudo, a TV volta a esperar: %v", c)
	}
}
