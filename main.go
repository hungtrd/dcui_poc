package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

type screenID int

const (
	scrDashboard screenID = iota
	scrDiagnostics
	scrRawRender
)

type focusID int

const (
	fIPv4 focusID = iota
	fPrefix
	fDNS
	fBtnApply
	fBtnClear
	fBtnQuit
	fCount // sentinel – used for modular wrap
)

type logLine struct {
	ts  time.Time
	msg string
}

type model struct {
	scr     screenID
	w, h    int
	inputs  []textinput.Model
	focus   focusID
	uni     bool // use unicode box-drawing for borders
	col     bool // use colors (false = forced monochrome)
	logs    []logLine
	term    string
	profile termenv.Profile
}

// ---------------------------------------------------------------------------
// ASCII-safe border (fallback when unicode is off)
// ---------------------------------------------------------------------------

var asciiBorder = lipgloss.Border{
	Top:          "-",
	Bottom:       "-",
	Left:         "|",
	Right:        "|",
	TopLeft:      "+",
	TopRight:     "+",
	BottomLeft:   "+",
	BottomRight:  "+",
	MiddleLeft:   "+",
	MiddleRight:  "+",
	Middle:       "+",
	MiddleTop:    "+",
	MiddleBottom: "+",
}

// ---------------------------------------------------------------------------
// Input validators
// ---------------------------------------------------------------------------

func validIPChars(s string) error {
	for _, r := range s {
		if r != '.' && (r < '0' || r > '9') {
			return fmt.Errorf("invalid char")
		}
	}
	return nil
}

func validDigits(s string) error {
	for _, r := range s {
		if r < '0' || r > '9' {
			return fmt.Errorf("invalid char")
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Model constructor
// ---------------------------------------------------------------------------

func initialModel() model {
	ip := textinput.New()
	ip.Placeholder = "192.168.1.1"
	ip.Prompt = "IPv4 Address : "
	ip.CharLimit = 15
	ip.Width = 15
	ip.Validate = validIPChars
	ip.Focus() // first field gets focus

	pfx := textinput.New()
	pfx.Placeholder = "24"
	pfx.Prompt = "Prefix       : "
	pfx.CharLimit = 2
	pfx.Width = 15
	pfx.Validate = validDigits

	dns := textinput.New()
	dns.Placeholder = "8.8.8.8"
	dns.Prompt = "DNS Server   : "
	dns.CharLimit = 15
	dns.Width = 15
	dns.Validate = validIPChars

	m := model{
		scr:    scrDashboard,
		w:      80,
		h:      24,
		inputs: []textinput.Model{ip, pfx, dns},
		focus:  fIPv4,
		uni:    true,
		col:    true,
		term:   os.Getenv("TERM"),
	}

	// Detect color profile from the environment / terminal.
	m.profile = termenv.EnvColorProfile()

	m.addLog("App started")
	return m
}

// ---------------------------------------------------------------------------
// Bubble Tea interface
// ---------------------------------------------------------------------------

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w = msg.Width
		m.h = msg.Height
		m.addLog(fmt.Sprintf("Resize %d×%d", m.w, m.h))
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// Forward non-key messages (e.g. cursor blink) to focused input.
	if m.scr == scrDashboard && m.focus <= fDNS {
		var cmd tea.Cmd
		m.inputs[m.focus], cmd = m.inputs[m.focus].Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m model) View() string {
	if m.w == 0 {
		return "Initializing…"
	}
	switch m.scr {
	case scrDiagnostics:
		return m.viewDiagnostics()
	case scrRawRender:
		return m.viewRawRender()
	default:
		return m.viewDashboard()
	}
}

// ---------------------------------------------------------------------------
// Key handling
// ---------------------------------------------------------------------------

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// ctrl+c always quits.
	if msg.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}

	// When a text input is focused on the dashboard, only intercept
	// navigation keys; everything else goes to the input widget.
	if m.scr == scrDashboard && m.focus <= fDNS {
		switch msg.Type {
		case tea.KeyTab:
			return m, m.advance(1)
		case tea.KeyShiftTab:
			return m, m.advance(-1)
		case tea.KeyEnter:
			return m, m.advance(1)
		case tea.KeyEsc:
			return m, nil // no-op on dashboard
		}
		// Forward to the text input.
		var cmd tea.Cmd
		m.inputs[m.focus], cmd = m.inputs[m.focus].Update(msg)
		return m, cmd
	}

	// Command keys – active when focus is on a button or on a non-dashboard
	// screen.
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "tab":
		if m.scr == scrDashboard {
			return m, m.advance(1)
		}
	case "shift+tab":
		if m.scr == scrDashboard {
			return m, m.advance(-1)
		}
	case "enter":
		if m.scr == scrDashboard {
			return m, m.pressButton()
		}
	case "d":
		if m.scr != scrDiagnostics {
			m.scr = scrDiagnostics
			m.addLog("Screen -> Diagnostics")
		}
	case "r":
		if m.scr != scrRawRender {
			m.scr = scrRawRender
			m.addLog("Screen -> Raw Render")
		}
	case "1":
		if m.scr != scrDashboard {
			m.scr = scrDashboard
			m.addLog("Screen -> Dashboard")
		}
	case "esc":
		if m.scr != scrDashboard {
			m.scr = scrDashboard
			m.addLog("Screen -> Dashboard")
		}
	case "u":
		m.uni = !m.uni
		m.addLog(fmt.Sprintf("Unicode borders %s", onOff(m.uni)))
	case "c":
		m.col = !m.col
		m.addLog(fmt.Sprintf("Colors %s", onOff(m.col)))
	case "l":
		m.logs = nil
		m.addLog("Log cleared")
	}
	return m, nil
}

// ---------------------------------------------------------------------------
// Focus management
// ---------------------------------------------------------------------------

// advance moves focus forward (dir=1) or backward (dir=-1), wrapping around.
func (m *model) advance(dir int) tea.Cmd {
	// Blur current text input.
	if m.focus <= fDNS {
		m.inputs[m.focus].Blur()
	}
	next := (int(m.focus) + dir + int(fCount)) % int(fCount)
	m.focus = focusID(next)
	var cmd tea.Cmd
	if m.focus <= fDNS {
		cmd = m.inputs[m.focus].Focus()
	}
	m.addLog(fmt.Sprintf("Focus -> %s", m.focusName()))
	return cmd
}

func (m model) focusName() string {
	switch m.focus {
	case fIPv4:
		return "IPv4"
	case fPrefix:
		return "Prefix"
	case fDNS:
		return "DNS"
	case fBtnApply:
		return "[Apply]"
	case fBtnClear:
		return "[Clear]"
	case fBtnQuit:
		return "[Quit]"
	}
	return "?"
}

// ---------------------------------------------------------------------------
// Button actions
// ---------------------------------------------------------------------------

func (m *model) pressButton() tea.Cmd {
	switch m.focus {
	case fBtnApply:
		m.addLog(fmt.Sprintf("APPLY ip=%s/%s dns=%s",
			m.inputs[0].Value(), m.inputs[1].Value(), m.inputs[2].Value()))
	case fBtnClear:
		for i := range m.inputs {
			m.inputs[i].SetValue("")
		}
		m.addLog("Form cleared")
	case fBtnQuit:
		return tea.Quit
	}
	return nil
}

// ---------------------------------------------------------------------------
// Logging (ring buffer, max 10)
// ---------------------------------------------------------------------------

func (m *model) addLog(msg string) {
	m.logs = append(m.logs, logLine{ts: time.Now(), msg: msg})
	if len(m.logs) > 10 {
		m.logs = m.logs[len(m.logs)-10:]
	}
}

// ---------------------------------------------------------------------------
// Style helpers
// ---------------------------------------------------------------------------

func (m model) curBorder() lipgloss.Border {
	if m.uni {
		return lipgloss.RoundedBorder()
	}
	return asciiBorder
}

func (m model) headerStyle() lipgloss.Style {
	s := lipgloss.NewStyle().Bold(true).Padding(0, 1)
	if m.col {
		s = s.Foreground(lipgloss.Color("15")).Background(lipgloss.Color("62"))
	} else {
		s = s.Reverse(true)
	}
	return s
}

func (m model) footerStyle() lipgloss.Style {
	s := lipgloss.NewStyle().Padding(0, 1)
	if m.col {
		s = s.Foreground(lipgloss.Color("252")).Background(lipgloss.Color("236"))
	} else {
		s = s.Faint(true)
	}
	return s
}

func (m model) panelStyle(width, height int) lipgloss.Style {
	s := lipgloss.NewStyle().
		Border(m.curBorder()).
		Width(width).
		Height(height).
		Padding(0, 1)
	if m.col {
		s = s.BorderForeground(lipgloss.Color("63"))
	}
	return s
}

func (m model) renderBtn(label string, focused bool) string {
	s := lipgloss.NewStyle().Padding(0, 1)
	if focused {
		if m.col {
			s = s.Bold(true).
				Foreground(lipgloss.Color("0")).
				Background(lipgloss.Color("12"))
		} else {
			s = s.Bold(true).Reverse(true)
		}
	} else {
		if m.col {
			s = s.Foreground(lipgloss.Color("7")).
				Background(lipgloss.Color("240"))
		} else {
			s = s.Faint(true)
		}
	}
	return s.Render(label)
}

func (m model) titleLabel(label string, active bool) string {
	s := lipgloss.NewStyle().Bold(true).Padding(0, 1)
	if m.col && active {
		s = s.Foreground(lipgloss.Color("212"))
	}
	return s.Render(label)
}

// ---------------------------------------------------------------------------
// Dashboard view
// ---------------------------------------------------------------------------

func (m model) viewDashboard() string {
	// ── header ──
	sep := " | "
	if m.uni {
		sep = " │ "
	}
	diagLine := strings.Join([]string{
		fmt.Sprintf("TERM=%s", m.term),
		m.profileStr(),
		fmt.Sprintf("%dx%d", m.w, m.h),
		fmt.Sprintf("Uni:%s", onOff(m.uni)),
		fmt.Sprintf("Col:%s", onOff(m.col)),
	}, sep)
	header := m.headerStyle().Width(m.w - 2).Render(
		"TUI Capability POC  " + diagLine)

	// ── panel dimensions ──
	// Each panel has: 1 border-left + 1 pad-left + content + 1 pad-right + 1 border-right = content + 4
	panelExtra := 4 // border (2) + padding (2)
	totalInner := m.w - panelExtra*2
	if totalInner < 20 {
		totalInner = 20
	}
	leftW := totalInner / 2
	rightW := totalInner - leftW

	bodyH := m.h - 4 // header ~1-2, footer ~1-2
	if bodyH < 8 {
		bodyH = 8
	}
	panelH := bodyH - 2 // subtract top+bottom border
	if panelH < 4 {
		panelH = 4
	}

	// ── left panel: form ──
	var left strings.Builder
	left.WriteString(m.titleLabel("Network Configuration", true))
	left.WriteString("\n\n")
	for i, inp := range m.inputs {
		left.WriteString(inp.View())
		if i < len(m.inputs)-1 {
			left.WriteString("\n")
		}
	}
	left.WriteString("\n\n")
	left.WriteString(m.renderBtn("Apply", m.focus == fBtnApply))
	left.WriteString(" ")
	left.WriteString(m.renderBtn("Clear", m.focus == fBtnClear))
	left.WriteString(" ")
	left.WriteString(m.renderBtn("Quit", m.focus == fBtnQuit))

	leftPanel := m.panelStyle(leftW, panelH).Render(left.String())

	// ── right panel: preview + log ──
	var right strings.Builder
	right.WriteString(m.titleLabel("Preview & Event Log", false))
	right.WriteString("\n\n")
	right.WriteString(fmt.Sprintf("IPv4   : %s\n", valOr(m.inputs[0].Value(), "-")))
	right.WriteString(fmt.Sprintf("Prefix : %s\n", valOr(m.inputs[1].Value(), "-")))
	right.WriteString(fmt.Sprintf("DNS    : %s\n", valOr(m.inputs[2].Value(), "-")))

	logSep := "── Log "
	if m.uni {
		logSep = "── Log ──────────────"
	} else {
		logSep = "-- Log ---------------"
	}
	right.WriteString("\n" + logSep + "\n")
	for _, l := range m.logs {
		right.WriteString(fmt.Sprintf("%s %s\n", l.ts.Format("15:04:05"), l.msg))
	}

	rightPanel := m.panelStyle(rightW, panelH).Render(right.String())

	// ── body ──
	body := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)

	// ── footer ──
	help := "Tab/S-Tab:focus  Enter:select  q:quit  d:diag  r:raw  u:unicode  c:colors  l:clear-log"
	footer := m.footerStyle().Width(m.w - 2).Render(help)

	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

// ---------------------------------------------------------------------------
// Diagnostics view
// ---------------------------------------------------------------------------

func (m model) viewDiagnostics() string {
	var b strings.Builder

	b.WriteString(m.headerStyle().Width(m.w - 2).Render("Diagnostics"))
	b.WriteString("\n\n")

	b.WriteString(fmt.Sprintf("  TERM            : %s\n", m.term))
	b.WriteString(fmt.Sprintf("  Window size     : %d x %d\n", m.w, m.h))
	b.WriteString(fmt.Sprintf("  Color profile   : %s\n", m.profileStr()))
	b.WriteString(fmt.Sprintf("  COLORTERM       : %s\n", os.Getenv("COLORTERM")))
	b.WriteString(fmt.Sprintf("  Unicode borders : %s\n", onOff(m.uni)))
	b.WriteString(fmt.Sprintf("  Colors enabled  : %s\n", onOff(m.col)))
	b.WriteString("\n")

	// ── basic colors 0-15 ──
	b.WriteString("  Basic colors (0-15):\n  ")
	for i := 0; i < 16; i++ {
		if m.col {
			s := lipgloss.NewStyle().Background(lipgloss.Color(fmt.Sprintf("%d", i)))
			b.WriteString(s.Render("  "))
		} else {
			b.WriteString(fmt.Sprintf("%2d", i))
		}
		if i == 7 {
			b.WriteString(" ")
		}
	}
	b.WriteString("\n\n")

	// ── 256-color sample ──
	b.WriteString("  256-color sample (16-231, every 6th):\n  ")
	count := 0
	for i := 16; i < 232; i += 6 {
		if m.col {
			s := lipgloss.NewStyle().Background(lipgloss.Color(fmt.Sprintf("%d", i)))
			b.WriteString(s.Render("  "))
		} else {
			b.WriteString("..")
		}
		count++
		if count%18 == 0 {
			b.WriteString("\n  ")
		}
	}
	b.WriteString("\n\n")

	// ── grayscale ramp ──
	b.WriteString("  Grayscale ramp (232-255):\n  ")
	for i := 232; i < 256; i++ {
		if m.col {
			s := lipgloss.NewStyle().Background(lipgloss.Color(fmt.Sprintf("%d", i)))
			b.WriteString(s.Render("  "))
		} else {
			b.WriteString("..")
		}
	}
	b.WriteString("\n\n")

	// ── unicode test ──
	b.WriteString("  Unicode rendering test:\n")
	if m.uni {
		b.WriteString("  Box single  : ┌─┬─┐ │ ├─┼─┤ │ └─┴─┘\n")
		b.WriteString("  Box rounded : ╭─╮ │ ╰─╯\n")
		b.WriteString("  Box double  : ╔═╗ ║ ╚═╝\n")
		b.WriteString("  Symbols     : ● ○ ■ □ ▲ ▼ ◆ ★ ✓ ✗\n")
		b.WriteString("  Block elems : ▀▄█▌▐░▒▓\n")
		b.WriteString("  Wide chars  : 你好世界\n")
		b.WriteString("  Arrows      : ← ↑ → ↓ ⇐ ⇑ ⇒ ⇓\n")
	} else {
		b.WriteString("  Box drawing : +-+-+ | +-+-+ | +-+-+\n")
		b.WriteString("  (ASCII mode – toggle with 'u')\n")
	}
	b.WriteString("\n")

	// ── footer ──
	help := "Esc/1:dashboard  u:toggle-unicode  c:toggle-colors  q:quit"
	b.WriteString(m.footerStyle().Width(m.w - 2).Render(help))
	return b.String()
}

// ---------------------------------------------------------------------------
// Raw Render Test view
// ---------------------------------------------------------------------------

func (m model) viewRawRender() string {
	var b strings.Builder

	b.WriteString(m.headerStyle().Width(m.w - 2).Render("Raw Render Test"))
	b.WriteString("\n\n")

	b.WriteString("  Text attribute tests:\n\n")

	sample := "The quick brown fox jumps over the lazy dog"
	attrs := []struct {
		name  string
		style lipgloss.Style
	}{
		{"Normal", lipgloss.NewStyle()},
		{"Bold", lipgloss.NewStyle().Bold(true)},
		{"Faint", lipgloss.NewStyle().Faint(true)},
		{"Italic", lipgloss.NewStyle().Italic(true)},
		{"Underline", lipgloss.NewStyle().Underline(true)},
		{"Strikethrough", lipgloss.NewStyle().Strikethrough(true)},
		{"Reverse", lipgloss.NewStyle().Reverse(true)},
		{"Blink", lipgloss.NewStyle().Blink(true)},
		{"Bold+Underline", lipgloss.NewStyle().Bold(true).Underline(true)},
	}
	for _, a := range attrs {
		b.WriteString(fmt.Sprintf("  %-16s: %s\n", a.name, a.style.Render(sample)))
	}
	b.WriteString("\n")

	// ── colored backgrounds ──
	if m.col {
		b.WriteString("  Background color samples:\n\n")
		bgTests := []struct {
			name string
			bg   string
			fg   string
		}{
			{"Subtle gray", "236", "252"},
			{"Hot pink", "205", "0"},
			{"Blue", "27", "15"},
			{"Green", "34", "15"},
			{"Yellow", "220", "0"},
			{"Red", "196", "15"},
			{"Cyan", "37", "0"},
			{"Purple", "93", "15"},
		}
		for _, t := range bgTests {
			s := lipgloss.NewStyle().
				Background(lipgloss.Color(t.bg)).
				Foreground(lipgloss.Color(t.fg)).
				Padding(0, 1)
			b.WriteString(fmt.Sprintf("  %-14s: %s\n", t.name, s.Render(t.name)))
		}
		b.WriteString("\n")

		// Combined styles
		combined := lipgloss.NewStyle().
			Bold(true).Underline(true).
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("62")).
			Padding(0, 1)
		b.WriteString("  Combined      : " + combined.Render("Bold + Underline + Color") + "\n")
	} else {
		b.WriteString("  (Colors disabled – press 'c' to enable)\n")
	}
	b.WriteString("\n")

	// ── border rendering ──
	b.WriteString("  Border test:\n\n")
	box := lipgloss.NewStyle().
		Border(m.curBorder()).
		Padding(0, 2)
	if m.col {
		box = box.BorderForeground(lipgloss.Color("63"))
	}
	boxContent := "This box tests border rendering.\nIt should have clean corners."
	// Indent the box
	for _, line := range strings.Split(box.Render(boxContent), "\n") {
		b.WriteString("  " + line + "\n")
	}
	b.WriteString("\n")

	// ── footer ──
	help := "Esc/1:dashboard  u:toggle-unicode  c:toggle-colors  q:quit"
	b.WriteString(m.footerStyle().Width(m.w - 2).Render(help))
	return b.String()
}

// ---------------------------------------------------------------------------
// Utility functions
// ---------------------------------------------------------------------------

func (m model) profileStr() string {
	switch m.profile {
	case termenv.TrueColor:
		return "TrueColor (16M)"
	case termenv.ANSI256:
		return "ANSI256 (256)"
	case termenv.ANSI:
		return "ANSI (16)"
	default:
		return "Ascii (no color)"
	}
}

func onOff(v bool) string {
	if v {
		return "ON"
	}
	return "OFF"
}

func valOr(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	m := initialModel()

	var opts []tea.ProgramOption
	if os.Getenv("NO_ALTSCREEN") != "1" {
		opts = append(opts, tea.WithAltScreen())
	}

	p := tea.NewProgram(m, opts...)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
