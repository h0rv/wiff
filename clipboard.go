package main

import (
	"encoding/base64"
	"os"
)

// copyToClipboard copies text to clipboard using OSC 52.
// Writes directly to /dev/tty to bypass tcell buffering.
// Returns true on success, false if the write failed.
func copyToClipboard(text string) bool {
	tty, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	defer func() { _ = tty.Close() }()

	encoded := base64.StdEncoding.EncodeToString([]byte(text))
	_, err = tty.WriteString("\033]52;c;" + encoded + "\a")
	return err == nil
}
