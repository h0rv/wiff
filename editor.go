package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// openInEditor suspends the TUI and opens the given file in the user's
// preferred editor ($EDITOR, $VISUAL, or "vi" as fallback). The optional
// lineNo places the cursor at that line (works with vim, nvim, nano, emacs, etc.).
// When the editor exits the TUI is resumed.
func openInEditor(s *State, file string, lineNo int) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = "vi"
	}

	// Resolve file relative to git repo root
	path := file
	if !filepath.IsAbs(path) {
		if root, err := gitRoot(); err == nil {
			path = filepath.Join(root, file)
		}
	}

	if _, err := os.Stat(path); err != nil {
		s.FlashMsg = fmt.Sprintf("File not found: %s", file)
		s.FlashExpiry = time.Now().Add(2 * time.Second)
		return
	}

	// Build args: editor +line file
	args := []string{}
	if lineNo > 0 {
		args = append(args, fmt.Sprintf("+%d", lineNo))
	}
	args = append(args, path)

	s.Screen.Fini()

	cmd := exec.Command(editor, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		s.FlashMsg = fmt.Sprintf("Editor error: %v", err)
		s.FlashExpiry = time.Now().Add(3 * time.Second)
	}

	// Resume TUI
	if err := s.Screen.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "Fatal: failed to reinitialize screen: %v\n", err)
		os.Exit(1)
	}
	s.Screen.Sync()
}

// gitRoot returns the top-level directory of the current git repository.
func gitRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", err
	}
	// Trim trailing newline
	root := string(out)
	if len(root) > 0 && root[len(root)-1] == '\n' {
		root = root[:len(root)-1]
	}
	return root, nil
}
