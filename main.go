package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
)

func main() {
	opts := parseArgs()

	screen, err := tcell.NewScreen()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create screen: %v\n", err)
		os.Exit(1)
	}
	if err := screen.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize screen: %v\n", err)
		os.Exit(1)
	}
	screen.EnableMouse()
	defer screen.Fini()
	w, h := screen.Size()
	state := &State{
		Refs:            opts.refs,
		Staged:          opts.staged,
		Screen:          screen,
		Width:           w,
		Height:          h,
		PipeMode:        isPipe(),
		SideBySide:      opts.sideBySide,
		LineNumbers:     !opts.noLineNumbers,
		ContextLines:    opts.contextLines,
		TreeOpen:        opts.explorer,
		TreeFocused:     opts.explorer,
		Wrap:            !opts.noWrap,
		SyntaxHighlight: !opts.noSyntax,
		DiffBg:          !opts.noDiffBg,
		WatchEnabled:    !isPipe(),
		Theme:           NewUITheme(opts.theme),
		HL:              NewHighlighter(),
	}
	state.HL.SetTheme(opts.theme)

	if err := loadDiff(state); err != nil {
		screen.Fini()
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	Render(state)

	if !state.PipeMode {
		go watchAndUpdate(state)
	}

	for {
		ev := screen.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventKey:
			if HandleKey(state, ev) {
				return
			}
			Render(state)
		case *tcell.EventMouse:
			switch ev.Buttons() {
			case tcell.WheelUp:
				state.ScrollBy(-3)
				Render(state)
			case tcell.WheelDown:
				state.ScrollBy(3)
				Render(state)
			case tcell.Button1:
				x, y := ev.Position()
				if state.TreeOpen && x < treeWidth {
					handleTreeClick(state, y)
				} else if y < state.Height-1 {
					HandleDiffClick(state, x, y)
				}
				Render(state)
			case tcell.Button3: // right-click
				x, y := ev.Position()
				if (!state.TreeOpen || x >= treeWidth) && y < state.Height-1 {
					HandleDiffRightClick(state, x, y)
				}
				Render(state)
			}
		case *tcell.EventResize:
			w, h := ev.Size()
			state.Width, state.Height = w, h
			state.BuildLines()
			state.ClampScroll()
			screen.Sync()
			Render(state)
		case *EventLabelTimeout:
			ResolvePendingLabel(state)
			Render(state)
		case *EventReload:
			if state.WatchEnabled {
				reloadDiff(state)
				Render(state)
			}
		}
	}
}

type cliOpts struct {
	refs          []string
	staged        bool
	sideBySide    bool
	noLineNumbers bool
	contextLines  int
	explorer      bool
	noWrap        bool
	noDiffBg      bool
	noSyntax      bool
	theme         string
}

func parseArgs() cliOpts {
	opts := cliOpts{contextLines: 3}
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-h" || arg == "--help":
			printUsage()
			os.Exit(0)
		case arg == "-v" || arg == "--version":
			fmt.Println("wiff " + version)
			os.Exit(0)
		case arg == "--themes":
			ListThemes()
		case arg == "--staged" || arg == "--cached":
			opts.staged = true
		case arg == "-t":
			if i+1 < len(args) {
				i++
				opts.theme = args[i]
			}
		case arg == "-s":
			opts.sideBySide = true
		case arg == "-e":
			opts.explorer = true
		case arg == "-W":
			opts.noWrap = true
		case arg == "-B":
			opts.noDiffBg = true
		case arg == "-S":
			opts.noSyntax = true
		case arg == "-N":
			opts.noLineNumbers = true
		case strings.HasPrefix(arg, "-U"):
			if n, err := strconv.Atoi(arg[2:]); err == nil && n >= 0 {
				opts.contextLines = n
			}
		default:
			opts.refs = append(opts.refs, arg)
		}
	}
	if opts.theme == "" {
		if env := os.Getenv("WIFF_THEME"); env != "" {
			opts.theme = env
		}
	}
	if opts.theme == "" {
		opts.theme = "monokai"
	}
	return opts
}

func printUsage() {
	fmt.Print(`wiff - a terminal diff viewer

Usage: wiff [flags] [ref] [ref2]

Flags:
  -s          Side-by-side mode
  -e          Open file explorer
  -N          Disable line numbers (on by default)
  -W          Disable line wrapping (on by default)
  -B          Disable diff background tints (on by default)
  -S          Disable syntax highlighting (on by default)
  -U<n>       Context lines (default 3)
  -t <name>   Color theme (default: monokai, env: WIFF_THEME)
  --staged    Show staged changes (same as --cached)
  --cached    Show staged changes (same as --staged)
  --themes    List available themes
  -v, --version  Show version
  -h, --help     Show this help

Arguments:
  ref         Git ref to diff against (default: unstaged changes)
  ref1 ref2   Diff between two refs

Examples:
  wiff              Show unstaged changes
  wiff HEAD         Diff against HEAD
  wiff HEAD~3       Diff against 3 commits ago
  wiff HEAD~3..HEAD Diff a commit range
  wiff main feature Diff between branches
  wiff --staged     Show staged changes
  wiff -s           Side-by-side mode
  git diff | wiff   Read diff from pipe

Keyboard Shortcuts:
  j/k         Scroll up/down          s   Toggle side-by-side
  d/u         Half page down/up       n   Toggle line numbers
  g/G         Jump to top/bottom      w   Toggle wrap
  ^D/^U       Half page down/up       e   Toggle file explorer
  +/-         More/less context       h   Toggle syntax highlight
  ]c/[c       Next/prev hunk          b   Toggle diff background
  ]f/[f       Next/prev file          /   Search
  Tab         Cycle to next file      W   Toggle watch mode
  Shift+Tab   Cycle to prev file      f   Full file view
  y+label     Yank added lines        o   Open in $EDITOR
  Y+label     Yank removed lines      F   Follow mode (watch)
  p+label     Yank patch              ?   Help overlay
  c+label     Copy result (new code)  q   Quit
  A+label     Stage/unstage hunk
`)
}

func isPipe() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) == 0
}

func loadDiff(s *State) error {
	var raw []byte
	var err error

	if s.PipeMode {
		raw, err = io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
	} else {
		raw, err = runGitDiff(s.Refs, s.ContextLines, s.Staged)
		if err != nil {
			return err
		}
	}

	hunks, err := parseDiff(raw)
	if err != nil {
		return err
	}
	s.Hunks = hunks
	buildTree(s)
	s.BuildLines()
	s.ClampScroll()

	return nil
}

func runGitDiff(refs []string, contextLines int, staged bool) ([]byte, error) {
	args := []string{"diff", "--no-color", fmt.Sprintf("-U%d", contextLines)}
	if staged {
		args = append(args, "--staged")
	}
	args = append(args, refs...)

	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			return nil, err
		}
	}
	return out, nil
}

// EventReload is a custom tcell event posted by the file watcher to trigger
// a diff reload on the main goroutine (avoids data races).
type EventReload struct {
	t time.Time
}

func (e *EventReload) When() time.Time { return e.t }

func watchAndUpdate(s *State) {
	updates := make(chan struct{}, 1)
	go startWatcher(updates)

	var pending *time.Timer
	for range updates {
		if pending != nil {
			pending.Stop()
		}
		pending = time.AfterFunc(300*time.Millisecond, func() {
			_ = s.Screen.PostEvent(&EventReload{t: time.Now()})
		})
	}
}

// hunkFingerprint returns a string that identifies a hunk by file and position.
func hunkFingerprint(h *Hunk) string {
	return fmt.Sprintf("%s:%d:%d", h.File, h.OldStart, h.NewStart)
}

// reloadDiff re-runs git diff and rebuilds display lines while preserving
// the user's scroll context (current file + approximate position).
func reloadDiff(s *State) {
	// Remember where the user is
	prevFile := s.CurrentFile()
	prevScroll := s.Scroll

	// Snapshot old hunks for follow mode comparison
	oldFingerprints := make(map[string]bool, len(s.Hunks))
	for i := range s.Hunks {
		oldFingerprints[hunkFingerprint(&s.Hunks[i])] = true
	}
	oldHunkCount := len(s.Hunks)

	raw, err := runGitDiff(s.Refs, s.ContextLines, s.Staged)
	if err != nil {
		return
	}
	hunks, err := parseDiff(raw)
	if err != nil {
		return
	}
	s.Hunks = hunks
	buildTree(s)
	s.BuildLines()

	// Follow mode: find first new hunk and scroll to it
	if s.FollowMode && len(s.Hunks) > 0 {
		newCount := len(s.Hunks) - oldHunkCount
		firstNewIdx := -1
		for i := range s.Hunks {
			if !oldFingerprints[hunkFingerprint(&s.Hunks[i])] {
				firstNewIdx = i
				break
			}
		}
		if firstNewIdx >= 0 && s.Hunks[firstNewIdx].StartLine >= 0 {
			s.Scroll = s.Hunks[firstNewIdx].StartLine
			s.ClampScroll()
			file := s.Hunks[firstNewIdx].File
			if newCount > 0 {
				s.FlashMsg = fmt.Sprintf("%d new hunks — %s", newCount, file)
			} else {
				s.FlashMsg = fmt.Sprintf("Changes in %s", file)
			}
			s.FlashExpiry = time.Now().Add(2 * time.Second)
			return
		}
	}

	// Try to restore scroll to the same file
	if prevFile != "" {
		for i, line := range s.Lines {
			if line.Style == StyleFileHeader && line.Text == prevFile {
				// Found the same file; restore relative offset
				s.Scroll = i
				s.ClampScroll()
				return
			}
		}
	}
	// File gone or not found: keep previous scroll, clamped
	s.Scroll = prevScroll
	s.ClampScroll()
}
