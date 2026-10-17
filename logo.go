package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Sponsor details. Shown at the end of a successful run and in --help.
const (
	sponsorName = "Wind VPN"
	sponsorURL  = "https://wind-vpn.space"
)

// logoLines is the "slant" figlet rendering of "golang-starter".
var logoLines = []string{
	"                __                             __             __           ",
	"   ____ _____  / /___ _____  ____ _      _____/ /_____ ______/ /____  _____",
	"  / __ `/ __ \\/ / __ `/ __ \\/ __ `/_____/ ___/ __/ __ `/ ___/ __/ _ \\/ ___/",
	" / /_/ / /_/ / / /_/ / / / / /_/ /_____(__  ) /_/ /_/ / /  / /_/  __/ /    ",
	" \\__, /\\____/_/\\__,_/_/ /_/\\__, /     /____/\\__/\\__,_/_/   \\__/\\___/_/     ",
	"/____/                    /____/                                           ",
}

const (
	colorGo     = "#00ADD8" // Go blue
	colorMint   = "#5DC9E2" // light gopher blue
	colorGreen  = "#3DDC97"
	colorDim    = "243"
	colorWarn   = "#F5A524"
	colorDanger = "#F31260"
)

var (
	styleDim     = lipgloss.NewStyle().Foreground(lipgloss.Color(colorDim))
	styleAccent  = lipgloss.NewStyle().Foreground(lipgloss.Color(colorGo)).Bold(true)
	styleOK      = lipgloss.NewStyle().Foreground(lipgloss.Color(colorGreen)).Bold(true)
	styleWarn    = lipgloss.NewStyle().Foreground(lipgloss.Color(colorWarn))
	styleErr     = lipgloss.NewStyle().Foreground(lipgloss.Color(colorDanger)).Bold(true)
	styleCmd     = lipgloss.NewStyle().Foreground(lipgloss.Color(colorMint))
	styleSection = lipgloss.NewStyle().Bold(true)
)

// logo renders the banner: a horizontal gradient from Go blue to green, a
// gopher and a tagline that plays on `go run`.
func logo() string {
	var b strings.Builder
	b.WriteString("\n")
	width := 0
	for _, l := range logoLines {
		if n := lipgloss.Width(l); n > width {
			width = n
		}
	}
	for _, l := range logoLines {
		b.WriteString(gradient(l, width, colorGo, colorGreen))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(styleAccent.Render("ʕ◔ϖ◔ʔ"))
	b.WriteString("  ")
	b.WriteString(styleSection.Render("ready"))
	b.WriteString(styleDim.Render(" · "))
	b.WriteString(styleSection.Render("set"))
	b.WriteString(styleDim.Render(" · "))
	b.WriteString(styleCmd.Render("go run"))
	b.WriteString(styleDim.Render("   fiber · templ · htmx, zero boilerplate"))
	b.WriteString("\n")
	return b.String()
}

// sponsorBox is the small, clearly labelled sponsor message.
func sponsorBox() string {
	title := styleDim.Render("sponsored · ") + styleAccent.Render(sponsorName)
	body := "Go compiles in a blink. Your traffic should move just as fast,\n" +
		"and stay private on the way. Let the wind carry your packets:\n" +
		styleCmd.Render(sponsorURL)
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(colorGo)).
		Padding(0, 2).
		Render(title + "\n\n" + body)
}

// gradient colours each rune of s with a linear blend of two hex colours.
func gradient(s string, width int, from, to string) string {
	fr, fg, fb := hexRGB(from)
	tr, tg, tb := hexRGB(to)
	var b strings.Builder
	i := 0
	for _, r := range s {
		t := 0.0
		if width > 1 {
			t = float64(i) / float64(width-1)
		}
		c := fmt.Sprintf("#%02x%02x%02x", lerp(fr, tr, t), lerp(fg, tg, t), lerp(fb, tb, t))
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(c)).Bold(true).Render(string(r)))
		i++
	}
	return b.String()
}

func hexRGB(h string) (int, int, int) {
	var r, g, b int
	fmt.Sscanf(strings.TrimPrefix(h, "#"), "%02x%02x%02x", &r, &g, &b)
	return r, g, b
}

func lerp(a, b int, t float64) int { return a + int(float64(b-a)*t+0.5) }
