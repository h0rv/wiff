package main

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/radovskyb/watcher"
)

// startWatcher watches for file changes in the git repo and sends
// notifications on updateCh. The .git directory is excluded.
func startWatcher(updateCh chan<- struct{}) {
	w := watcher.New()
	w.SetMaxEvents(1)
	w.FilterOps(watcher.Write, watcher.Create, watcher.Remove, watcher.Rename)

	root, err := gitRoot()
	if err != nil || root == "" {
		return
	}

	w.AddFilterHook(func(_ os.FileInfo, fullPath string) error {
		if strings.Contains(fullPath, string(filepath.Separator)+".git"+string(filepath.Separator)) ||
			strings.HasSuffix(fullPath, string(filepath.Separator)+".git") {
			return watcher.ErrSkip
		}
		return nil
	})

	if err := w.AddRecursive(root); err != nil {
		return
	}

	go func() {
		for {
			select {
			case <-w.Event:
				select {
				case updateCh <- struct{}{}:
				default:
				}
			case <-w.Error:
				return
			case <-w.Closed:
				return
			}
		}
	}()

	_ = w.Start(100 * time.Millisecond)
}
