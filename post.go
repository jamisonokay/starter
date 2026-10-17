package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	alpineURL    = "https://cdn.jsdelivr.net/npm/alpinejs@3.x.x/dist/cdn.min.js"
	shadcnPkg    = "github.com/axadrn/shadcn-templ/v2/cmd/shadcn-templ"
	shadcnAdd    = "button card" // components the generated home page uses
	tailwindBase = "https://github.com/tailwindlabs/tailwindcss/releases/latest/download/"
)

// step prints a section header.
func step(title string) {
	fmt.Println("\n" + styleAccent.Render("▸ ") + styleSection.Render(title))
}

func warnf(format string, args ...any) {
	fmt.Fprintln(os.Stderr, "\n"+styleWarn.Render("⚠ ")+fmt.Sprintf(format, args...))
}

// postSteps runs everything that has to happen after the files are written.
func postSteps(o *Options, d *Data, dir string) error {
	step("Initialising Go module")
	if err := runIn(dir, "go", "mod", "init", d.Module); err != nil {
		return fmt.Errorf("go mod init: %w", err)
	}
	if o.SkipInstall {
		fmt.Println(styleDim.Render("\n(--skip-install: dependencies, tools, downloads and docker were skipped)"))
		return nil
	}

	// Order matters: `go mod tidy` fails while views/**/*_templ.go do not exist
	// yet, so add templ first, generate, and only then tidy.
	step("Installing templ")
	if err := runIn(dir, "go", "get", "github.com/a-h/templ@latest"); err != nil {
		return fmt.Errorf("go get templ: %w", err)
	}
	templ, err := installTempl(dir)
	if err != nil {
		return err
	}

	if d.Alpine {
		step("Downloading Alpine.js")
		if err := downloadAlpine(dir); err != nil {
			warnf("alpine.js: %v\n  Run `make alpine` later.", err)
		}
	}

	// shadcn components are generated into views/ui, which the layout and the
	// home page import, so this must run before `templ generate`.
	shadcnOK := true
	if d.Shadcn {
		step("Setting up shadcn/ui (shadcn-templ)")
		if err := setupShadcn(dir); err != nil {
			shadcnOK = false
			warnf("shadcn/ui: %v\n  The project cannot compile until the components exist. After fixing the problem run:\n"+
				"    make ui-init && make setup", err)
		}
	}

	if shadcnOK {
		step("Generating templ code and tidying modules")
		if err := runIn(dir, templ, "generate"); err != nil {
			return fmt.Errorf("templ generate: %w", err)
		}
		if err := runIn(dir, "go", "mod", "tidy"); err != nil {
			return fmt.Errorf("go mod tidy: %w", err)
		}
	}

	if d.Tailwind {
		step("Setting up Tailwind CSS")
		if err := setupTailwind(dir); err != nil {
			// Not fatal: the project is generated, CSS can be built later with `make css`.
			warnf("tailwind: %v\n  Run `make setup` later.", err)
		}
	}

	if d.React {
		step("Setting up React (Vite + TypeScript)")
		if err := setupReact(dir); err != nil {
			warnf("react: %v\n  Run `make react-install react-build` later.", err)
		}
	}

	if d.Compose {
		if err := ensureDocker(o.Yes); err != nil {
			warnf("docker: %v", err)
		}
	}
	return nil
}

// installTempl installs the templ CLI in the same version as the library that
// was just added to go.mod (a mismatch makes templ refuse to compile).
func installTempl(dir string) (string, error) {
	ver := "latest"
	if out, err := outputIn(dir, "go", "list", "-m", "-f", "{{.Version}}", "github.com/a-h/templ"); err == nil {
		if v := strings.TrimSpace(out); v != "" {
			ver = v
		}
	}
	if err := runIn(dir, "go", "install", "github.com/a-h/templ/cmd/templ@"+ver); err != nil {
		return "", fmt.Errorf("go install templ: %w", err)
	}
	bin := filepath.Join(goBinDir(dir), "templ"+exeSuffix())
	if _, err := os.Stat(bin); err != nil {
		return "", fmt.Errorf("templ was installed but not found in %s", bin)
	}
	return bin, nil
}

func goBinDir(dir string) string {
	if out, err := outputIn(dir, "go", "env", "GOBIN"); err == nil && strings.TrimSpace(out) != "" {
		return strings.TrimSpace(out)
	}
	out, _ := outputIn(dir, "go", "env", "GOPATH")
	gopath := strings.TrimSpace(out)
	if i := strings.IndexRune(gopath, os.PathListSeparator); i >= 0 {
		gopath = gopath[:i]
	}
	return filepath.Join(gopath, "bin")
}

// ------------------------------------------------------------------ alpine

func downloadAlpine(dir string) error {
	target := filepath.Join(dir, "static", "js", "alpine.min.js")
	fmt.Printf("%s\n", styleDim.Render("→ "+alpineURL))
	return downloadFile(alpineURL, target, 0o644, time.Minute)
}

// ------------------------------------------------------------------ shadcn

// setupShadcn installs the shadcn-templ CLI and lets it write the theme into
// web/input.css and the components into views/ui. components.json (rendered
// from the template) already points the aliases into views/, and `init --force`
// keeps those aliases.
func setupShadcn(dir string) error {
	if err := runIn(dir, "go", "install", shadcnPkg+"@latest"); err != nil {
		return fmt.Errorf("go install shadcn-templ: %w", err)
	}
	bin := filepath.Join(goBinDir(dir), "shadcn-templ"+exeSuffix())
	if _, err := os.Stat(bin); err != nil {
		return fmt.Errorf("shadcn-templ was installed but not found in %s", bin)
	}
	if err := runIn(dir, bin, "init", "--force", "--css", "web/input.css"); err != nil {
		return fmt.Errorf("shadcn-templ init: %w", err)
	}
	args := append([]string{"add"}, strings.Fields(shadcnAdd)...)
	if err := runIn(dir, bin, args...); err != nil {
		return fmt.Errorf("shadcn-templ add: %w", err)
	}
	// Always writes views/ui/scripts_bundle.go and the hashed JS bundle.
	if err := runIn(dir, bin, "bundle"); err != nil {
		return fmt.Errorf("shadcn-templ bundle: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------- tailwind

func setupTailwind(dir string) error {
	bin, err := downloadTailwind(dir)
	if err != nil {
		return err
	}
	fmt.Println(styleDim.Render("→ building the first static/css/app.css"))
	return runIn(dir, bin, "-i", "web/input.css", "-o", "static/css/app.css", "--minify")
}

// tailwindAsset maps the current platform to the release asset name.
func tailwindAsset() (string, error) {
	var arch string
	switch runtime.GOARCH {
	case "amd64":
		arch = "x64"
	case "arm64":
		arch = "arm64"
	default:
		return "", fmt.Errorf("no tailwindcss build for architecture %s", runtime.GOARCH)
	}
	switch runtime.GOOS {
	case "linux":
		return "tailwindcss-linux-" + arch, nil
	case "darwin":
		return "tailwindcss-macos-" + arch, nil
	case "windows":
		return "tailwindcss-windows-" + arch + ".exe", nil
	}
	return "", fmt.Errorf("no tailwindcss build for OS %s", runtime.GOOS)
}

func downloadTailwind(dir string) (string, error) {
	asset, err := tailwindAsset()
	if err != nil {
		return "", err
	}
	url := tailwindBase + asset
	target := filepath.Join(dir, "bin", "tailwindcss"+exeSuffix())
	fmt.Println(styleDim.Render("→ " + url))
	if err := downloadFile(url, target, 0o755, 5*time.Minute); err != nil {
		return "", err
	}
	return target, nil
}

// ------------------------------------------------------------------- react

func setupReact(dir string) error {
	if !hasCmd("npm") {
		return fmt.Errorf("`npm` was not found in PATH (install Node.js 20+: https://nodejs.org)")
	}
	front := filepath.Join(dir, "frontend")
	if err := runIn(front, "npm", "install"); err != nil {
		return fmt.Errorf("npm install: %w", err)
	}
	if err := runIn(front, "npm", "run", "build"); err != nil {
		return fmt.Errorf("npm run build: %w", err)
	}
	return nil
}

// ------------------------------------------------------------------ docker

func ensureDocker(yes bool) error {
	if _, err := exec.LookPath("docker"); err == nil {
		return checkDocker()
	}

	if runtime.GOOS != "linux" {
		return fmt.Errorf("docker is not installed. Install Docker Desktop: https://docs.docker.com/get-docker/")
	}

	// Never install system software unattended: --yes only skips questions.
	if yes {
		return fmt.Errorf("docker is not installed: install it (https://docs.docker.com/get-docker/) before running `make db-up`")
	}
	fmt.Println("\nDocker was not found. It can be installed with the official script https://get.docker.com (needs sudo).")
	if !confirm("Install Docker now?", true) {
		return fmt.Errorf("docker is not installed: `make db-up` will not work until you install it")
	}

	var fetch string
	switch {
	case hasCmd("curl"):
		fetch = "curl -fsSL https://get.docker.com"
	case hasCmd("wget"):
		fetch = "wget -qO- https://get.docker.com"
	default:
		return fmt.Errorf("curl or wget is required to download the install script")
	}
	// sudo must apply to `sh` (which does the installing), not to curl.
	sh := "sh"
	if os.Geteuid() != 0 {
		if !hasCmd("sudo") {
			return fmt.Errorf("root or sudo is required to install Docker")
		}
		sh = "sudo sh"
	}
	if err := runIn(".", "sh", "-c", fetch+" | "+sh); err != nil {
		return fmt.Errorf("docker installation: %w", err)
	}

	fmt.Println(`
Docker is installed. To run docker without sudo, add yourself to the group and log in again:
  sudo usermod -aG docker $USER && newgrp docker`)
	return nil
}

// checkDocker verifies the daemon is reachable and docker compose v2 exists.
func checkDocker() error {
	if err := exec.Command("docker", "info").Run(); err != nil {
		return fmt.Errorf("docker is installed but the daemon is unreachable (not running, or no permission).\n" +
			"  Linux: sudo systemctl start docker && sudo usermod -aG docker $USER && newgrp docker")
	}
	if err := exec.Command("docker", "compose", "version").Run(); err != nil {
		return fmt.Errorf("the `docker compose` (v2) plugin was not found: https://docs.docker.com/compose/install/")
	}
	return nil
}

// --------------------------------------------------------------- exec utils

// downloadFile fetches url into target atomically (temp file + rename).
func downloadFile(url, target string, mode os.FileMode, timeout time.Duration) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	resp, err := (&http.Client{Timeout: timeout}).Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s returned %s", url, resp.Status)
	}
	tmp, err := os.CreateTemp(filepath.Dir(target), ".download-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op after a successful rename
	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), mode); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), target)
}

func runIn(dir, name string, args ...string) error {
	fmt.Println(styleDim.Render(fmt.Sprintf("$ %s %s", filepath.Base(name), strings.Join(args, " "))))
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout, cmd.Stderr, cmd.Stdin = os.Stdout, os.Stderr, os.Stdin
	return cmd.Run()
}

func outputIn(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return string(out), err
}

func hasCmd(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func exeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

// confirm asks a yes/no question on a plain terminal (used outside the TUI).
func confirm(q string, def bool) bool {
	hint := "Y/n"
	if !def {
		hint = "y/N"
	}
	fmt.Printf("%s [%s]: ", q, hint)
	var s string
	if _, err := fmt.Scanln(&s); err != nil {
		return def
	}
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "y", "yes":
		return true
	case "n", "no":
		return false
	}
	return def
}
