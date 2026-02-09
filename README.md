# TUI Capability POC

A single-binary Go terminal UI app for validating rendering and interactivity
on low-capability consoles (Proxmox VGA `TERM=linux`, serial `TERM=vt100`, etc.).

Built with Charmbracelet Bubble Tea, Bubbles, Lipgloss, and termenv.

## Build

```bash
# Static binary — no libc, no CGO, copy anywhere
CGO_ENABLED=0 go build -o tui-capability-poc .
```

## Run

```bash
./tui-capability-poc
```

### Environment variables

| Variable | Effect |
|---|---|
| `NO_ALTSCREEN=1` | Disable alternate screen buffer (useful if your terminal doesn't support it) |
| `TERM=linux` | Simulate Proxmox VGA console (8 colors) |
| `TERM=vt100` | Simulate basic serial console |
| `COLORTERM=truecolor` | Force truecolor detection |

### Simulating low-capability terminals

```bash
# Proxmox VGA console (8 colors, no unicode)
TERM=linux ./tui-capability-poc

# Serial console
TERM=vt100 ./tui-capability-poc

# No alternate screen (raw scrollback)
NO_ALTSCREEN=1 ./tui-capability-poc

# Combine: serial + no alt screen
TERM=vt100 NO_ALTSCREEN=1 ./tui-capability-poc
```

## Screens

### 1 — Dashboard (default)

- **Header**: title + live diagnostics (TERM, color profile, window size, toggle states)
- **Left panel**: network config form (IPv4, Prefix, DNS) with Apply/Clear/Quit buttons
- **Right panel**: live preview of form values + scrolling event log (last 10 entries)
- **Footer**: keybinding reference

### 2 — Diagnostics (`d`)

- TERM, window size, color profile, COLORTERM
- Color test grids: basic 0-15, 256-color sample, grayscale ramp
- Unicode rendering test: box drawing, rounded/double borders, symbols, wide chars, arrows

### 3 — Raw Render Test (`r`)

- Text attributes: normal, bold, faint, italic, underline, strikethrough, reverse, blink
- Colored background samples (8 colors when enabled)
- Combined style test (bold + underline + color)
- Border rendering with current unicode/ASCII mode

## Keybindings

| Key | Action |
|---|---|
| `Tab` / `Shift+Tab` | Cycle focus through form fields and buttons |
| `Enter` | Activate focused button / advance to next field |
| `q` | Quit (when not typing in a text field) |
| `Ctrl+C` | Quit (always) |
| `d` | Switch to Diagnostics screen |
| `r` | Switch to Raw Render Test screen |
| `1` / `Esc` | Back to Dashboard |
| `u` | Toggle Unicode / ASCII borders |
| `c` | Toggle colors on/off (force monochrome) |
| `l` | Clear event log |

When a text input is focused, letter keys go to the input. Only Tab, Shift+Tab,
Enter, Esc, and Ctrl+C are intercepted.

## Input validation

- **IPv4 / DNS**: digits and dots only, max 15 characters
- **Prefix**: digits only, max 2 characters (0–32)

## What to look for on low-capability terminals

1. **Diagnostics screen** — does the detected color profile match expectations?
2. **Color grids** — do blocks render or show as garbage?
3. **Unicode test** — do box-drawing chars align? Do wide chars break layout?
4. **Toggle `u`** — ASCII borders should always render cleanly
5. **Toggle `c`** — monochrome mode uses only bold/faint/reverse (no ANSI colors)
6. **Resize** — shrink/grow the terminal; check the log for resize events
7. **Raw Render** — which text attributes actually work on your terminal?
