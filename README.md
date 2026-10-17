# golang-starter

```
                __                             __             __
   ____ _____  / /___ _____  ____ _      _____/ /_____ ______/ /____  _____
  / __ `/ __ \/ / __ `/ __ \/ __ `/_____/ ___/ __/ __ `/ ___/ __/ _ \/ ___/
 / /_/ / /_/ / / /_/ / / / / /_/ /_____(__  ) /_/ /_/ / /  / /_/  __/ /
 \__, /\____/_/\__,_/_/ /_/\__, /     /____/\__/\__,_/_/   \__/\___/_/
/____/                    /____/
ʕ◔ϖ◔ʔ  ready · set · go run   fiber · templ · htmx, zero boilerplate
```

An interactive project generator for **Go + Fiber + templ + htmx**. Pick only what you need:

| | |
| --- | --- |
| **Always included** | [Fiber v3](https://github.com/gofiber/fiber), [templ](https://templ.guide), [htmx](https://htmx.org) |
| **Optional** | PostgreSQL (pgx) · Redis · WebSocket ([`gofiber/contrib/v3/websocket`](https://github.com/gofiber/contrib/tree/main/v3/websocket)) · Tailwind CSS · Alpine.js · [shadcn/ui](https://github.com/axadrn/shadcn-templ) · React (Vite + TypeScript) |

The template lives in `template/` of this repository. The CLI downloads the repository as a tarball,
asks what you want in a [Bubble Tea](https://github.com/charmbracelet/bubbletea) UI and renders only the selected layers.

```bash
go install github.com/jamisonokay/starter@latest
starter
```

> The installed binary is named after the last segment of the module path (`starter`).
> Rename the repository to `golang-starter` (and the module path with it) to get a `golang-starter` binary.

## Usage

```bash
starter                                     # fully interactive
starter github.com/me/shop                  # name from the argument, features are asked
starter github.com/me/shop --postgres --websocket --tailwind --alpine   # only confirm
starter github.com/me/shop --react --yes    # no questions at all
```

Interactive keys: `↑/↓` move · `space` toggle · `enter` continue · `esc` back · `ctrl+c` quit.
Without a terminal (CI, pipes) the CLI never asks anything and uses the flags.

| Flag | Meaning |
| --- | --- |
| `--name` | project name / module path (`go mod init <name>`) |
| `--dir` | target directory (default: last segment of the name) |
| `--postgres`, `--redis` | add the service (+ `docker-compose.yml`, `make db-*`, `.env`, Go code) |
| `--websocket` | Fiber WebSocket endpoint `/ws`, a broadcast hub and a live demo |
| `--tailwind` | Tailwind CSS (standalone CLI downloaded into `./bin`, no Node.js) |
| `--alpine` | Alpine.js, downloaded into `static/js/alpine.min.js` (`make alpine`) |
| `--shadcn` | shadcn/ui components for templ; **implies `--tailwind`** |
| `--react` | React + TypeScript + Vite in `./frontend`, built into `static/react/app.js` |
| `--yes` | never ask; Docker is never installed unattended |
| `--skip-install` | only generate files and `go mod init` (no network); run `make setup` later |
| `--repo`, `--ref` | another template repository / branch / tag |
| `--template-dir` | local `template/` directory (template development) |

> **Breaking change:** Tailwind used to be on by default (`--tailwind=false` to opt out). It is now opt-in.

## What the CLI does

1. Asks for the project name and optional features (or reads flags).
2. Downloads `template/` and applies the layers in order.
3. `go mod init` → `go get templ` → `go install templ@<same version>` → *(Alpine download)* →
   *(shadcn-templ init/add/bundle)* → `templ generate` → `go mod tidy`.
4. Tailwind: downloads the standalone binary for your OS/arch and builds the first `static/css/app.css`.
5. React: `npm install` and `npm run build` (typecheck + Vite).
6. Postgres/Redis: checks Docker (on Linux it offers to install it, never with `--yes`).
7. Writes `.env` from `.env.example`.

Optional steps that need the network (Alpine, Tailwind, npm) only warn when they fail, and the generated
project stays usable: `make setup` repeats them.

## Generated project

All UI lives in `views/`:

```
views/
  layouts/       <html> shell, scripts and styles
  pages/         one templ per page
  components/    reusable pieces and the demo cards
  ui/            shadcn/ui components   (only with shadcn)
  utils/         shadcn/ui helpers      (only with shadcn)
frontend/        React + TypeScript + Vite (only with react)
```

Details (make targets, config, React mounting, shadcn usage) are in the generated project's own `README.md`.

## Template layout

```
template/
  base/        always: cmd, internal, views, static, Makefile, Dockerfile ...
  plaincss/    static/css/app.css           (when Tailwind is NOT selected)
  tailwind/    web/input.css                (Tailwind)
  alpine/      Alpine demo component
  shadcn/      components.json, views/ui/scripts.templ, shadcn demo component
  postgres/    internal/db (pgxpool)
  redis/       internal/cache (go-redis v9)
  websocket/   internal/ws hub, internal/handlers/ws.go, demo component + JS
  react/       frontend/ (Vite + TS), internal/handlers/api.go, demo component
  compose/     docker-compose.yml           (postgres or redis)
```

* Files ending in `.tmpl` go through `text/template` and lose the suffix; everything else is copied verbatim
  (so `.templ`, JS and Makefile syntax never clash). **Go files in `template/` are `.go.tmpl`** so that
  the generator module contains no foreign packages.
* Template data: `.Module`, `.Name`, `.DBName`, `.GoVersion`, `.Postgres`, `.Redis`, `.WebSocket`,
  `.Tailwind`, `.Alpine`, `.Shadcn`, `.React`, `.Compose`.
* Generated `.go` files are `gofmt`-ed; a syntax error in a template stops the generation.
* A new option = a new layer directory + a line in `layers()` + an entry in `featureList` (`tui.go`) + a flag in `parseFlags()`.

## Development

```bash
go test .       # renders all 128 feature combinations and drives the TUI model
go build -o /tmp/gs . && /tmp/gs demo --yes --websocket --tailwind --react --template-dir ./template
```

## Sponsor

CLI sponsored by **[Wind VPN](https://wind-vpn.space)**: fast, private traffic. Go compiles in a blink, let the wind carry your packets.
