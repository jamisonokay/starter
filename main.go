// golang-starter scaffolds a Go + Fiber + templ + htmx project.
//
// Fiber, templ and htmx are always included. Everything else (PostgreSQL,
// Redis, WebSocket, Tailwind CSS, Alpine.js, shadcn/ui, React) is optional and
// is picked in an interactive terminal UI or with flags.
//
// The template files live in the `template/` directory of a GitHub repository.
// The CLI downloads that repository as a tarball and renders only the layers
// that were selected.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// Override at build time:
//
//	go build -ldflags "-X main.defaultRepo=owner/repo -X main.defaultRef=main"
var (
	defaultRepo = "jamisonokay/starter"
	defaultRef  = "main"
)

// Options are the answers of the user (from flags or the interactive UI).
type Options struct {
	Module string // go module path, e.g. github.com/me/app
	Dir    string // target directory

	// Optional features. Fiber, templ and htmx are always on.
	Postgres  bool
	Redis     bool
	WebSocket bool
	Tailwind  bool // Tailwind CSS (standalone CLI downloaded into ./bin)
	Alpine    bool // Alpine.js downloaded into static/js/alpine.min.js
	Shadcn    bool // shadcn/ui components for templ (needs Tailwind)
	React     bool // React + TypeScript + Vite in ./frontend

	Yes         bool // never prompt, use flags/defaults
	SkipInstall bool // do not run go get/tidy, templ, tailwind, npm, docker ...
	Repo        string
	Ref         string
	TemplateDir string // use a local template instead of GitHub
}

func main() {
	opts, set := parseFlags(os.Args[1:])
	if err := run(opts, set); err != nil {
		if errors.Is(err, errCancelled) {
			fmt.Fprintln(os.Stderr, styleDim.Render("\nCancelled. Nothing was created."))
			os.Exit(130)
		}
		fmt.Fprintln(os.Stderr, "\n"+styleErr.Render("error:")+" "+err.Error())
		os.Exit(1)
	}
}

func parseFlags(args []string) (*Options, map[string]bool) {
	o := &Options{}
	fs := flag.NewFlagSet("golang-starter", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: golang-starter [project-name] [flags]

Always included: Fiber v3, templ, htmx.
Run without feature flags to pick optional features interactively.

`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nSponsored by %s: %s\n", sponsorName, sponsorURL)
	}
	fs.StringVar(&o.Module, "name", "", "project name / go module path (go mod init <name>)")
	fs.StringVar(&o.Dir, "dir", "", "project directory (default: last segment of the name)")
	fs.BoolVar(&o.Postgres, "postgres", false, "add PostgreSQL (pgx) + docker compose")
	fs.BoolVar(&o.Redis, "redis", false, "add Redis + docker compose")
	fs.BoolVar(&o.WebSocket, "websocket", false, "add Fiber WebSocket (github.com/gofiber/contrib/v3/websocket) with a demo hub")
	fs.BoolVar(&o.Tailwind, "tailwind", false, "add Tailwind CSS (standalone CLI is downloaded into ./bin)")
	fs.BoolVar(&o.Alpine, "alpine", false, "add Alpine.js (downloaded into static/js/alpine.min.js)")
	fs.BoolVar(&o.Shadcn, "shadcn", false, "add shadcn/ui for templ (github.com/axadrn/shadcn-templ), implies --tailwind")
	fs.BoolVar(&o.React, "react", false, "add React + TypeScript + Vite in ./frontend")
	fs.BoolVar(&o.Yes, "yes", false, "never ask questions, use flag values")
	fs.BoolVar(&o.SkipInstall, "skip-install", false, "only generate files and run go mod init (no network, no installs)")
	fs.StringVar(&o.Repo, "repo", defaultRepo, "GitHub repository with the template (owner/name)")
	fs.StringVar(&o.Ref, "ref", defaultRef, "branch, tag or commit of the template repository")
	fs.StringVar(&o.TemplateDir, "template-dir", "", "use a local template/ directory instead of GitHub (template development)")

	// The standard flag package stops at the first positional argument, so
	// `golang-starter myapp --postgres` needs a second pass.
	var positional []string
	for {
		_ = fs.Parse(args)
		if fs.NArg() == 0 {
			break
		}
		positional = append(positional, fs.Arg(0))
		args = fs.Args()[1:]
	}
	if o.Module == "" && len(positional) > 0 {
		o.Module = positional[0]
	}

	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	return o, set
}

// anyFeatureFlag reports whether the user picked features on the command line.
func anyFeatureFlag(set map[string]bool) bool {
	for _, k := range []string{"postgres", "redis", "websocket", "tailwind", "alpine", "shadcn", "react"} {
		if set[k] {
			return true
		}
	}
	return false
}

// normalize resolves dependencies between features and tells the user about it.
func normalize(o *Options) (notes []string) {
	if o.Shadcn && !o.Tailwind {
		o.Tailwind = true
		notes = append(notes, "shadcn/ui is built on Tailwind CSS, so Tailwind was enabled as well.")
	}
	return notes
}

func run(o *Options, set map[string]bool) error {
	if _, err := exec.LookPath("go"); err != nil {
		return errors.New("`go` was not found in PATH: install Go (https://go.dev/dl/)")
	}

	interactive := !o.Yes && isTerminal()

	fmt.Print(logo())
	if interactive {
		if err := runTUI(o, set); err != nil {
			return err
		}
	} else if strings.TrimSpace(o.Module) == "" {
		return errors.New("specify a project name: golang-starter <name> or --name (no terminal for interactive mode)")
	}
	o.Module = strings.TrimSpace(o.Module)

	for _, n := range normalize(o) {
		fmt.Println(styleWarn.Render("note: ") + n)
	}

	data, err := newData(o)
	if err != nil {
		return err
	}
	dir := o.Dir
	if dir == "" {
		dir = data.Name
	}
	if dir, err = filepath.Abs(dir); err != nil {
		return err
	}
	if err := ensureEmptyDir(dir); err != nil {
		return err
	}

	printSummary(o, data, dir)

	src, cleanup, err := prepareTemplate(o)
	if err != nil {
		return err
	}
	defer cleanup()

	step("Generating project")
	if err := render(src, layers(data), dir, data); err != nil {
		return err
	}
	if err := createDotEnv(dir); err != nil {
		return err
	}

	if err := postSteps(o, data, dir); err != nil {
		return err
	}

	printNext(o, data, dir)
	return nil
}

// Data is what the .tmpl files see.
type Data struct {
	Module    string
	Name      string // last path segment of Module
	DBName    string // Name sanitised for use as a database name
	GoVersion string // major.minor of the Go toolchain, e.g. "1.26"

	Postgres  bool
	Redis     bool
	WebSocket bool
	Tailwind  bool
	Alpine    bool
	Shadcn    bool
	React     bool

	Compose bool // docker-compose.yml is generated
}

var moduleRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._~/-]*$`)

// validateModule is shared by the CLI flow and the interactive input.
func validateModule(m string) error {
	m = strings.TrimSpace(m)
	if m == "" {
		return errors.New("a project name is required")
	}
	if !moduleRe.MatchString(m) || strings.Contains(m, "//") || strings.HasSuffix(m, "/") {
		return fmt.Errorf("invalid module name %q: use letters, digits and . _ - /", m)
	}
	return nil
}

func newData(o *Options) (*Data, error) {
	if err := validateModule(o.Module); err != nil {
		return nil, err
	}
	name := path.Base(o.Module)
	return &Data{
		Module:    o.Module,
		Name:      name,
		DBName:    dbName(name),
		GoVersion: goVersion(),
		Postgres:  o.Postgres,
		Redis:     o.Redis,
		WebSocket: o.WebSocket,
		Tailwind:  o.Tailwind,
		Alpine:    o.Alpine,
		Shadcn:    o.Shadcn,
		React:     o.React,
		Compose:   o.Postgres || o.Redis,
	}, nil
}

var nonIdent = regexp.MustCompile(`[^a-z0-9_]+`)

func dbName(name string) string {
	s := strings.Trim(nonIdent.ReplaceAllString(strings.ToLower(name), "_"), "_")
	if s == "" || (s[0] >= '0' && s[0] <= '9') {
		s = "app_" + s
	}
	return s
}

var goVersionRe = regexp.MustCompile(`^go(\d+\.\d+)`)

// goVersion returns the major.minor version of the installed toolchain (used
// for the Dockerfile base image). It falls back to a recent release.
func goVersion() string {
	out, err := exec.Command("go", "env", "GOVERSION").Output()
	if err == nil {
		if m := goVersionRe.FindStringSubmatch(strings.TrimSpace(string(out))); m != nil {
			return m[1]
		}
	}
	return "1.26"
}

// layers lists template sub-directories in the order they are applied.
// A layer's files may overwrite files of earlier layers.
func layers(d *Data) []string {
	l := []string{"base"}
	if d.Tailwind {
		l = append(l, "tailwind")
	} else {
		l = append(l, "plaincss")
	}
	if d.Alpine {
		l = append(l, "alpine")
	}
	if d.Shadcn {
		l = append(l, "shadcn")
	}
	if d.Postgres {
		l = append(l, "postgres")
	}
	if d.Redis {
		l = append(l, "redis")
	}
	if d.WebSocket {
		l = append(l, "websocket")
	}
	if d.React {
		l = append(l, "react")
	}
	if d.Compose {
		l = append(l, "compose")
	}
	return l
}

func ensureEmptyDir(dir string) error {
	entries, err := os.ReadDir(dir)
	switch {
	case os.IsNotExist(err):
		return nil
	case err != nil:
		return err
	case len(entries) > 0:
		abs, _ := filepath.Abs(dir)
		return fmt.Errorf("directory %s already exists and is not empty", abs)
	}
	return nil
}

// featureNames lists the enabled features in display order.
func featureNames(d *Data) []string {
	names := []string{"Fiber", "templ", "htmx"}
	add := func(on bool, n string) {
		if on {
			names = append(names, n)
		}
	}
	add(d.Postgres, "PostgreSQL")
	add(d.Redis, "Redis")
	add(d.WebSocket, "WebSocket")
	add(d.Tailwind, "Tailwind CSS")
	add(d.Alpine, "Alpine.js")
	add(d.Shadcn, "shadcn/ui")
	add(d.React, "React + Vite + TS")
	return names
}

func printSummary(o *Options, d *Data, dir string) {
	fmt.Println()
	row := func(k, v string) { fmt.Printf("  %s %s\n", styleDim.Render(fmt.Sprintf("%-9s", k)), v) }
	row("project", styleSection.Render(d.Module))
	row("folder", dir)
	row("stack", strings.Join(featureNames(d), styleDim.Render(" · ")))
	if o.TemplateDir != "" {
		row("template", o.TemplateDir+" (local)")
	} else {
		row("template", fmt.Sprintf("github.com/%s@%s", o.Repo, o.Ref))
	}
}

func printNext(o *Options, d *Data, dir string) {
	fmt.Println()
	fmt.Println(styleOK.Render("✔ Done!") + " Next steps:")
	fmt.Println()
	fmt.Println("  " + styleCmd.Render("cd "+dir))
	if o.SkipInstall {
		fmt.Println("  " + styleCmd.Render("make setup") + styleDim.Render("    # install tools and dependencies (skipped by --skip-install)"))
	}
	if d.Compose {
		fmt.Println("  " + styleCmd.Render("make db-up") + styleDim.Render("   # start "+services(d)+" in Docker"))
	}
	fmt.Println("  " + styleCmd.Render("make dev") + styleDim.Render("     # live reload on http://localhost:5173 (app on :3000)"))
	fmt.Println()
	fmt.Println(sponsorBox())
	fmt.Println()
}

func services(d *Data) string {
	switch {
	case d.Postgres && d.Redis:
		return "Postgres and Redis"
	case d.Postgres:
		return "Postgres"
	default:
		return "Redis"
	}
}
