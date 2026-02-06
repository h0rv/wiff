package main

// KeyBinding defines a single application key binding.
type KeyBinding struct {
	Key  rune
	Name string
}

// All application keybindings. Adding a key here automatically reserves it
// so it won't be used as a hunk label.
var keyBindings = []KeyBinding{
	// Navigation
	{Key: 'j', Name: "scroll down"},
	{Key: 'k', Name: "scroll up"},
	{Key: 'd', Name: "half page down"},
	{Key: 'u', Name: "half page up"},
	{Key: 'g', Name: "go to top"},
	{Key: 'G', Name: "go to bottom"},

	// Modes & toggles
	{Key: 's', Name: "side-by-side"},
	{Key: 'n', Name: "line numbers / next match"},
	{Key: 'w', Name: "wrap"},
	{Key: 'e', Name: "explorer"},
	{Key: 'h', Name: "syntax highlight"},
	{Key: 'b', Name: "diff background"},

	// Full file view
	{Key: 'f', Name: "toggle full file view"},

	// Yank / patch / copy (pending key prefixes)
	{Key: 'y', Name: "yank added"},
	{Key: 'Y', Name: "yank removed"},
	{Key: 'p', Name: "yank patch"},
	{Key: 'c', Name: "copy result"},

	// Staging
	{Key: 'A', Name: "stage/unstage hunk"},

	// Follow mode
	{Key: 'F', Name: "follow mode"},

	// Search
	{Key: '/', Name: "search"},
	{Key: 'N', Name: "prev search match"},

	// Hunk / file navigation (pending key prefixes)
	{Key: ']', Name: "next hunk/file"},
	{Key: '[', Name: "prev hunk/file"},

	// Tree mode
	{Key: 'a', Name: "show all (tree)"},

	// Help
	{Key: '?', Name: "help"},

	// Actions
	{Key: 'o', Name: "open in editor"},

	// Watch mode
	{Key: 'W', Name: "toggle watch mode"},

	// Misc
	{Key: 'q', Name: "quit"},
	{Key: '+', Name: "more context"},
	{Key: '=', Name: "more context"},
	{Key: '-', Name: "less context"},
}

// reservedKeys is derived from keyBindings. Any rune here is skipped for hunk labels.
var reservedKeys map[rune]bool

// availableLabels is the list of safe label characters: a-z then A-Z, minus reserved.
var availableLabels []rune

func init() {
	reservedKeys = make(map[rune]bool, len(keyBindings))
	for _, kb := range keyBindings {
		reservedKeys[kb.Key] = true
	}
	// Lowercase first, then uppercase for overflow
	for r := 'a'; r <= 'z'; r++ {
		if !reservedKeys[r] {
			availableLabels = append(availableLabels, r)
		}
	}
	for r := 'A'; r <= 'Z'; r++ {
		if !reservedKeys[r] {
			availableLabels = append(availableLabels, r)
		}
	}
}
