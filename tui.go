package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var errCancelled = errors.New("cancelled")

// isTerminal reports whether both stdin and stdout are interactive terminals.
func isTerminal() bool {
	in, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	out, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return in.Mode()&os.ModeCharDevice != 0 && out.Mode()&os.ModeCharDevice != 0
}

// feature is one optional checkbox in the UI.
type feature struct {
	id    string
	title string
	desc  string
	flag  func(*Options) *bool
}

var featureList = []feature{
	{"postgres", "PostgreSQL", "pgx connection pool + docker compose", func(o *Options) *bool { return &o.Postgres }},
	{"redis", "Redis", "go-redis client + docker compose", func(o *Options) *bool { return &o.Redis }},
	{"websocket", "WebSocket", "Fiber WebSocket hub with a live demo", func(o *Options) *bool { return &o.WebSocket }},
	{"tailwind", "Tailwind CSS", "standalone CLI, no Node.js needed", func(o *Options) *bool { return &o.Tailwind }},
	{"alpine", "Alpine.js", "tiny reactive layer, saved to static/js", func(o *Options) *bool { return &o.Alpine }},
	{"shadcn", "shadcn/ui", "shadcn-templ components (needs Tailwind)", func(o *Options) *bool { return &o.Shadcn }},
	{"react", "React", "Vite + TypeScript in ./frontend", func(o *Options) *bool { return &o.React }},
}

type stage int

const (
	stageName stage = iota
	stageFeatures
	stageConfirm
	stageDone
)

type model struct {
	opts         *Options
	stage        stage
	skipFeatures bool // features were passed as flags
	input        textinput.Model
	err          string // validation error under the name input
	notice       string // transient hint under the feature list
	cursor       int
	confirmYes   bool
	confirmed    bool
	cancelled    bool
}

// runTUI asks for everything that was not passed as a flag. It mutates o.
func runTUI(o *Options, set map[string]bool) error {
	ti := textinput.New()
	ti.Placeholder = "github.com/you/my-app"
	ti.Prompt = "› "
	ti.PromptStyle = styleAccent
	ti.CharLimit = 120
	ti.Width = 48
	ti.SetValue(strings.TrimSpace(o.Module))
	ti.CursorEnd()

	m := model{
		opts:         o,
		input:        ti,
		skipFeatures: anyFeatureFlag(set),
		confirmYes:   true,
	}
	if validateModule(o.Module) == nil {
		m.stage = stageFeatures
		if m.skipFeatures {
			m.stage = stageConfirm
		}
	} else {
		m.stage = stageName
		m.input.Focus()
	}

	final, err := tea.NewProgram(m).Run()
	if err != nil {
		return err
	}
	fm := final.(model)
	if fm.cancelled || !fm.confirmed {
		return errCancelled
	}
	return nil
}

func (m model) Init() tea.Cmd {
	if m.stage == stageName {
		return textinput.Blink
	}
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, isKey := msg.(tea.KeyMsg)
	if isKey && key.String() == "ctrl+c" {
		m.cancelled = true
		return m, tea.Quit
	}

	switch m.stage {
	case stageName:
		return m.updateName(msg, key, isKey)
	case stageFeatures:
		if isKey {
			return m.updateFeatures(key)
		}
	case stageConfirm:
		if isKey {
			return m.updateConfirm(key)
		}
	}
	return m, nil
}

func (m model) updateName(msg tea.Msg, key tea.KeyMsg, isKey bool) (tea.Model, tea.Cmd) {
	if isKey {
		switch key.String() {
		case "esc":
			m.cancelled = true
			return m, tea.Quit
		case "enter":
			name := strings.TrimSpace(m.input.Value())
			if err := validateModule(name); err != nil {
				m.err = err.Error()
				return m, nil
			}
			m.err = ""
			m.opts.Module = name
			m.input.Blur()
			if m.skipFeatures {
				m.stage = stageConfirm
			} else {
				m.stage = stageFeatures
			}
			return m, nil
		}
		m.err = ""
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m model) updateFeatures(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.notice = ""
	switch key.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		} else {
			m.cursor = len(featureList) - 1
		}
	case "down", "j":
		if m.cursor < len(featureList)-1 {
			m.cursor++
		} else {
			m.cursor = 0
		}
	case " ", "x":
		m.toggle(featureList[m.cursor].id)
	case "enter":
		m.stage = stageConfirm
	case "esc", "shift+tab":
		m.stage = stageName
		m.input.Focus()
		return m, textinput.Blink
	}
	return m, nil
}

// toggle flips a feature and keeps dependencies consistent.
func (m *model) toggle(id string) {
	var f *feature
	for i := range featureList {
		if featureList[i].id == id {
			f = &featureList[i]
		}
	}
	p := f.flag(m.opts)
	*p = !*p
	switch {
	case id == "shadcn" && *p && !m.opts.Tailwind:
		m.opts.Tailwind = true
		m.notice = "shadcn/ui is built on Tailwind CSS, so Tailwind was enabled too."
	case id == "tailwind" && !*p && m.opts.Shadcn:
		m.opts.Shadcn = false
		m.notice = "shadcn/ui needs Tailwind CSS, so it was turned off too."
	}
}

func (m model) updateConfirm(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "left", "right", "h", "l", "tab":
		m.confirmYes = !m.confirmYes
	case "y", "Y":
		m.confirmed, m.stage = true, stageDone
		return m, tea.Quit
	case "n", "N":
		m.cancelled = true
		return m, tea.Quit
	case "enter":
		if m.confirmYes {
			m.confirmed, m.stage = true, stageDone
		} else {
			m.cancelled = true
		}
		return m, tea.Quit
	case "esc", "shift+tab":
		if m.skipFeatures {
			m.stage = stageName
			m.input.Focus()
			return m, textinput.Blink
		}
		m.stage = stageFeatures
	}
	return m, nil
}

// ------------------------------------------------------------------- views

func (m model) View() string {
	switch m.stage {
	case stageName:
		return m.viewName()
	case stageFeatures:
		return m.viewFeatures()
	case stageConfirm:
		return m.viewConfirm()
	}
	return "" // stageDone: clear the frame, the summary is printed afterwards
}

func (m model) viewName() string {
	var b strings.Builder
	b.WriteString("\n" + styleSection.Render("What are you building?") + "\n")
	b.WriteString(styleDim.Render("Project name, used as the Go module path (go mod init).") + "\n\n")
	b.WriteString("  " + m.input.View() + "\n")
	if m.err != "" {
		b.WriteString("\n  " + styleErr.Render("✗ ") + m.err + "\n")
	}
	b.WriteString("\n" + help("enter continue", "esc quit"))
	return b.String()
}

func (m model) viewFeatures() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(styleSection.Render("Always included") + "  ")
	b.WriteString(styleAccent.Render("Fiber v3") + styleDim.Render(" · ") +
		styleAccent.Render("templ") + styleDim.Render(" · ") +
		styleAccent.Render("htmx") + "\n\n")
	b.WriteString(styleSection.Render("Optional features") + "\n\n")

	titleW := 0
	for _, f := range featureList {
		if n := lipgloss.Width(f.title); n > titleW {
			titleW = n
		}
	}
	for i, f := range featureList {
		on := *f.flag(m.opts)
		cursor := "  "
		if i == m.cursor {
			cursor = styleAccent.Render("❯ ")
		}
		box := styleDim.Render("[ ]")
		if on {
			box = styleOK.Render("[✓]")
		}
		title := f.title + strings.Repeat(" ", titleW-lipgloss.Width(f.title))
		if i == m.cursor {
			title = styleAccent.Render(title)
		} else if on {
			title = styleSection.Render(title)
		}
		b.WriteString(fmt.Sprintf(" %s%s %s  %s\n", cursor, box, title, styleDim.Render(f.desc)))
	}
	if m.notice != "" {
		b.WriteString("\n  " + styleWarn.Render("! "+m.notice) + "\n")
	}
	b.WriteString("\n" + help("↑/↓ move", "space toggle", "enter continue", "esc back", "ctrl+c quit"))
	return b.String()
}

func (m model) viewConfirm() string {
	d := &Data{
		Postgres: m.opts.Postgres, Redis: m.opts.Redis, WebSocket: m.opts.WebSocket,
		Tailwind: m.opts.Tailwind || m.opts.Shadcn, Alpine: m.opts.Alpine,
		Shadcn: m.opts.Shadcn, React: m.opts.React,
	}
	var b strings.Builder
	b.WriteString("\n" + styleSection.Render("Ready to scaffold") + "\n\n")
	b.WriteString("  " + styleDim.Render("project ") + styleSection.Render(m.opts.Module) + "\n")
	b.WriteString("  " + styleDim.Render("stack   ") + strings.Join(featureNames(d), styleDim.Render(" · ")) + "\n\n")

	yes, no := " Yes ", " No "
	on := lipgloss.NewStyle().Background(lipgloss.Color(colorGo)).Foreground(lipgloss.Color("#000000")).Bold(true)
	if m.confirmYes {
		yes = on.Render(yes)
		no = styleDim.Render(no)
	} else {
		yes = styleDim.Render(yes)
		no = on.Render(no)
	}
	b.WriteString("  Create the project?  " + yes + "  " + no + "\n")
	b.WriteString("\n" + help("←/→ choose", "enter confirm", "y/n", "esc back"))
	return b.String()
}

func help(items ...string) string {
	return styleDim.Render(strings.Join(items, " · ")) + "\n"
}
