package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func renderOpts(t *testing.T, o *Options) string {
	t.Helper()
	normalize(o)
	if o.Module == "" {
		o.Module = "github.com/me/My-App"
	}
	dst := t.TempDir()
	d, err := newData(o)
	if err != nil {
		t.Fatal(err)
	}
	if err := render(os.DirFS("template"), layers(d), dst, d); err != nil {
		t.Fatal(err)
	}
	return dst
}

func read(t *testing.T, dir, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func exists(dir, rel string) bool {
	_, err := os.Stat(filepath.Join(dir, rel))
	return err == nil
}

func TestRenderCombinations(t *testing.T) {
	cases := []struct {
		name         string
		opts         Options
		wantCompose  bool
		mustContain  map[string][]string
		mustNotExist []string
	}{
		{"core only", Options{}, false,
			map[string][]string{
				"cmd/main.go":               {`"github.com/me/My-App/internal/config"`, "github.com/gofiber/fiber/v3"},
				"internal/config/config.go": {`env("PORT", "3000")`},
				"views/layouts/base.templ":  {"/static/js/htmx.min.js", `class="page"`},
				"views/pages/home.templ":    {"@components.HtmxDemo()"},
				"static/css/app.css":        {".panel"},
				"Makefile":                  {"templ generate --watch"},
			},
			[]string{"docker-compose.yml", "internal/db", "internal/cache", "internal/ws", "frontend", "web/input.css",
				"components.json", "views/ui", "static/js/ws-demo.js"}},
		{"tailwind", Options{Tailwind: true}, false,
			map[string][]string{
				"web/input.css": {`@import "tailwindcss"`, "@theme static"},
				"Makefile":      {"TAILWIND ?= ./bin/tailwindcss", "--watch=always", "bin/tailwindcss:"},
				".gitignore":    {"static/css/app.css"},
				"Dockerfile":    {"tailwindcss"},
			},
			[]string{"static/css/app.css"}},
		{"postgres", Options{Postgres: true}, true,
			map[string][]string{
				"cmd/main.go":        {"db.Connect", "pool.Close()"},
				".env":               {"DATABASE_URL=postgres://postgres:postgres@localhost:5432/my_app"},
				"Makefile":           {"db-up:"},
				"docker-compose.yml": {"postgres:17-alpine", "postgres_data:"},
			},
			[]string{"internal/cache"}},
		{"redis", Options{Redis: true}, true,
			map[string][]string{
				"cmd/main.go":        {"cache.Connect"},
				".env":               {"REDIS_URL=redis://localhost:6379/0"},
				"docker-compose.yml": {"redis:7-alpine", "redis_data:"},
			},
			[]string{"internal/db"}},
		{"websocket", Options{WebSocket: true}, false,
			map[string][]string{
				"cmd/main.go":                {"ws.NewHub()"},
				"internal/handlers/route.go": {"h.registerWS(app)"},
				"internal/handlers/ws.go":    {"github.com/gofiber/contrib/v3/websocket"},
				"views/pages/home.templ":     {"@components.WebSocketDemo()"},
				"static/js/ws-demo.js":       {"new WebSocket"},
			},
			[]string{"frontend"}},
		{"alpine", Options{Alpine: true}, false,
			map[string][]string{
				"views/layouts/base.templ":           {"/static/js/alpine.min.js"},
				"views/components/alpine_demo.templ": {"x-data"},
				"Makefile":                           {"alpine:", "static/js/alpine.min.js"},
			},
			nil},
		{"shadcn implies tailwind", Options{Shadcn: true}, false,
			map[string][]string{
				"components.json":            {`"ui": "github.com/me/My-App/views/ui"`, `"dir": "static/js"`},
				"views/layouts/base.templ":   {"@ui.Scripts()", `class="h-full dark"`},
				"views/ui/scripts.templ":     {"templ Scripts()"},
				"views/ui/scripts_bundle.go": {"const bundleSrc"},
				"web/input.css":              {"shadcn-templ-*.js"},
				"Makefile":                   {"ui-add:", "$(SHADCN) bundle", "dev-ui:"},
				"Dockerfile":                 {"shadcn-templ bundle"},
				".gitignore":                 {"static/js/shadcn-templ-*.js"},
			},
			[]string{"static/css/app.css"}},
		{"react", Options{React: true}, false,
			map[string][]string{
				"frontend/package.json":      {`"name": "my_app-frontend"`, `"vite"`},
				"frontend/vite.config.ts":    {`outDir: "../static/react"`},
				"frontend/src/main.tsx":      {"data-react"},
				"internal/handlers/api.go":   {`"/api"`},
				"internal/handlers/route.go": {"h.registerAPI(app)"},
				"views/layouts/base.templ":   {"/static/react/app.js"},
				"Dockerfile":                 {"FROM node:22-alpine AS frontend", "COPY --from=frontend"},
				".gitignore":                 {"frontend/node_modules/", "static/react/"},
			},
			nil},
		{"everything", Options{Postgres: true, Redis: true, WebSocket: true, Tailwind: true, Alpine: true, Shadcn: true, React: true}, true,
			map[string][]string{
				"internal/handlers/handlers.go": {"*pgxpool.Pool", "*redis.Client", "*ws.Hub"},
				"internal/config/config.go":     {"DatabaseURL", "RedisURL"},
				"docker-compose.yml":            {"postgres:", "redis:"},
				"views/pages/home.templ":        {"ShadcnDemo", "ReactDemo", "WebSocketDemo", "AlpineDemo"},
			}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			o := c.opts
			dst := renderOpts(t, &o)
			if err := createDotEnv(dst); err != nil {
				t.Fatal(err)
			}
			if exists(dst, "docker-compose.yml") != c.wantCompose {
				t.Errorf("docker-compose.yml exists=%v, want %v", !c.wantCompose, c.wantCompose)
			}
			for f, subs := range c.mustContain {
				if !exists(dst, f) {
					t.Errorf("%s must exist", f)
					continue
				}
				got := read(t, dst, f)
				for _, s := range subs {
					if !strings.Contains(got, s) {
						t.Errorf("%s: missing %q\n%s", f, s, got)
					}
				}
			}
			for _, f := range c.mustNotExist {
				if exists(dst, f) {
					t.Errorf("%s must not exist", f)
				}
			}
		})
	}
}

// Every one of the 2^7 feature combinations must render without leftover
// template actions or Go syntax errors (render gofmt-checks every .go file).
func TestAllCombinationsRender(t *testing.T) {
	for mask := 0; mask < 1<<7; mask++ {
		o := Options{
			Postgres: mask&1 != 0, Redis: mask&2 != 0, WebSocket: mask&4 != 0, Tailwind: mask&8 != 0,
			Alpine: mask&16 != 0, Shadcn: mask&32 != 0, React: mask&64 != 0,
		}
		dst := renderOpts(t, &o)
		_ = filepath.WalkDir(dst, func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || strings.HasSuffix(p, ".png") || strings.HasSuffix(p, ".ico") || strings.HasSuffix(p, "htmx.min.js") {
				return nil
			}
			b, _ := os.ReadFile(p)
			// ws-demo.js and JSON legitimately have braces, but never "{{" followed by a Go-template verb.
			for _, bad := range []string{"{{if", "{{-", "{{.", "{{end", "{{else"} {
				if strings.Contains(string(b), bad) {
					t.Errorf("mask %07b: %s contains unrendered %q", mask, p, bad)
				}
			}
			return nil
		})
	}
}

func TestNormalizeShadcnNeedsTailwind(t *testing.T) {
	o := &Options{Shadcn: true}
	if notes := normalize(o); !o.Tailwind || len(notes) != 1 {
		t.Fatalf("tailwind=%v notes=%v", o.Tailwind, notes)
	}
}

func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "space":
		return tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func send(m tea.Model, keys ...string) tea.Model {
	for _, k := range keys {
		m, _ = m.Update(key(k))
	}
	return m
}

func TestTUIFlow(t *testing.T) {
	o := &Options{}
	ti := textinput.New()
	ti.Focus()
	var m tea.Model = model{opts: o, input: ti, stage: stageName, confirmYes: true}

	m = send(m, "enter") // empty name is rejected
	if m.(model).stage != stageName || m.(model).err == "" {
		t.Fatal("empty name should stay on the name step with an error")
	}
	m = send(m, "m", "y", "a", "p", "p", "enter")
	if m.(model).stage != stageFeatures || o.Module != "myapp" {
		t.Fatalf("stage=%v module=%q", m.(model).stage, o.Module)
	}

	// features: postgres(0) redis(1) websocket(2) tailwind(3) alpine(4) shadcn(5) react(6)
	m = send(m, "space")                                // postgres on
	m = send(m, "down", "down", "down", "down", "down") // -> shadcn
	m = send(m, "space")                                // shadcn on => tailwind auto-on
	if !o.Postgres || !o.Shadcn || !o.Tailwind || m.(model).notice == "" {
		t.Fatalf("opts=%+v notice=%q", o, m.(model).notice)
	}
	m = send(m, "up", "up") // -> tailwind
	m = send(m, "space")    // tailwind off => shadcn off
	if o.Tailwind || o.Shadcn {
		t.Fatalf("turning tailwind off must turn shadcn off: %+v", o)
	}

	m = send(m, "enter") // -> confirm
	if m.(model).stage != stageConfirm {
		t.Fatal("expected confirm step")
	}
	m = send(m, "enter")
	fm := m.(model)
	if !fm.confirmed || fm.cancelled {
		t.Fatalf("confirmed=%v cancelled=%v", fm.confirmed, fm.cancelled)
	}
}

func TestTUICancel(t *testing.T) {
	var m tea.Model = model{opts: &Options{Module: "x"}, stage: stageConfirm, confirmYes: true}
	m = send(m, "n")
	if !m.(model).cancelled {
		t.Fatal("n must cancel")
	}
}

func TestParseFlagsPositionalAnywhere(t *testing.T) {
	for _, args := range [][]string{{"myapp", "--postgres", "--react"}, {"--postgres", "myapp", "--react"}, {"--name", "myapp", "--postgres", "--react"}} {
		o, set := parseFlags(args)
		if o.Module != "myapp" || !o.Postgres || !o.React || o.Tailwind || !set["react"] || !anyFeatureFlag(set) {
			t.Errorf("%v -> %+v %v", args, o, set)
		}
	}
	if _, set := parseFlags([]string{"myapp"}); anyFeatureFlag(set) {
		t.Error("no feature flags were passed")
	}
}

func TestLogoWidth(t *testing.T) {
	for _, l := range logoLines {
		if lipgloss.Width(l) > 80 {
			t.Errorf("logo line wider than 80 columns: %q", l)
		}
	}
	if !strings.Contains(sponsorBox(), "wind-vpn.space") {
		t.Error("sponsor box must contain the sponsor URL")
	}
}

func TestDBName(t *testing.T) {
	for in, want := range map[string]string{"My-App": "my_app", "123": "app_123", "shop.v2": "shop_v2", "--": "app_"} {
		if got := dbName(in); got != want {
			t.Errorf("dbName(%q)=%q want %q", in, got, want)
		}
	}
}

func tarball(files map[string]string) *bytes.Reader {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		_ = tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg})
		_, _ = tw.Write([]byte(body))
	}
	tw.Close()
	gz.Close()
	return bytes.NewReader(buf.Bytes())
}

func TestExtractTemplate(t *testing.T) {
	dst := t.TempDir()
	err := extractTemplate(tarball(map[string]string{
		"owner-repo-abc123/README.md":              "ignored",
		"owner-repo-abc123/template/base/Makefile": "ok",
		"owner-repo-abc123/template/redis/a/b.go":  "ok",
	}), dst)
	if err != nil {
		t.Fatal(err)
	}
	if !exists(dst, "base/Makefile") || !exists(dst, "redis/a/b.go") || exists(dst, "README.md") {
		t.Fatal("unexpected extraction result")
	}
}

func TestExtractRejectsTraversal(t *testing.T) {
	err := extractTemplate(tarball(map[string]string{"x/template/../../evil": "boom"}), t.TempDir())
	if err == nil {
		t.Fatal("expected error for path traversal")
	}
}

func TestExtractNoTemplateDir(t *testing.T) {
	if err := extractTemplate(tarball(map[string]string{"x/README.md": "hi"}), t.TempDir()); err == nil {
		t.Fatal("expected error when template/ is missing")
	}
}
