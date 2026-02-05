package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
)

const treeWidth = 30

// TreeFile represents a changed file in the tree view
type TreeFile struct {
	Path    string
	Added   int
	Removed int
}

// TreeNode is a flattened entry for rendering the tree sidebar.
// It can be a directory or a file leaf.
type TreeNode struct {
	Display string // text to display (dir name with prefix, or filename)
	Path    string // full file path (only set for file leaves)
	Depth   int    // indentation depth
	IsDir   bool
	Added   int
	Removed int
}

// dirNode is an intermediate tree structure used to build the hierarchy.
type dirNode struct {
	name     string
	children map[string]*dirNode
	files    []*TreeFile
	order    []string // insertion-ordered child keys
}

func newDirNode(name string) *dirNode {
	return &dirNode{
		name:     name,
		children: make(map[string]*dirNode),
	}
}

func (d *dirNode) getOrCreateChild(name string) *dirNode {
	if c, ok := d.children[name]; ok {
		return c
	}
	c := newDirNode(name)
	d.children[name] = c
	d.order = append(d.order, name)
	return c
}

// buildTree computes tree file stats from hunks
func buildTree(s *State) {
	type stats struct{ add, rem int }
	m := make(map[string]*stats)
	var order []string

	for _, h := range s.Hunks {
		if _, ok := m[h.File]; !ok {
			m[h.File] = &stats{}
			order = append(order, h.File)
		}
		for _, l := range h.Lines {
			switch l.Op {
			case '+':
				m[h.File].add++
			case '-':
				m[h.File].rem++
			}
		}
	}

	s.TreeFiles = make([]TreeFile, 0, len(order))
	for _, path := range order {
		st := m[path]
		s.TreeFiles = append(s.TreeFiles, TreeFile{
			Path:    path,
			Added:   st.add,
			Removed: st.rem,
		})
	}

	s.TreeNodes = buildTreeNodes(s.TreeFiles)
}

// buildTreeNodes converts flat file list into a hierarchical tree,
// collapsing single-child directory chains.
func buildTreeNodes(files []TreeFile) []TreeNode {
	if len(files) == 0 {
		return nil
	}

	// Build intermediate tree
	root := newDirNode("")
	for i := range files {
		tf := &files[i]
		parts := strings.Split(tf.Path, "/")
		node := root
		for _, part := range parts[:len(parts)-1] {
			node = node.getOrCreateChild(part)
		}
		node.files = append(node.files, tf)
	}

	// Flatten with collapsing
	var nodes []TreeNode
	var flatten func(n *dirNode, depth int, prefix string)
	flatten = func(n *dirNode, depth int, prefix string) {
		// Sort children: directories first, then files, both alphabetically
		dirKeys := make([]string, len(n.order))
		copy(dirKeys, n.order)
		sort.Strings(dirKeys)

		sortedFiles := make([]*TreeFile, len(n.files))
		copy(sortedFiles, n.files)
		sort.Slice(sortedFiles, func(i, j int) bool {
			// Extract basename for sorting
			return basename(sortedFiles[i].Path) < basename(sortedFiles[j].Path)
		})

		// Process child directories
		for _, key := range dirKeys {
			child := n.children[key]
			dirPath := prefix + key + "/"

			// Collapse single-child chains:
			// If this dir has exactly one child dir and no files, merge them
			collapsed := child
			collapsedName := key
			for len(collapsed.children) == 1 && len(collapsed.files) == 0 {
				for subKey, subChild := range collapsed.children {
					collapsedName += "/" + subKey
					collapsed = subChild
				}
			}

			nodes = append(nodes, TreeNode{
				Display: collapsedName + "/",
				Depth:   depth,
				IsDir:   true,
			})

			flatten(collapsed, depth+1, dirPath)
		}

		// Process files at this level
		for _, tf := range sortedFiles {
			nodes = append(nodes, TreeNode{
				Display: basename(tf.Path),
				Path:    tf.Path,
				Depth:   depth,
				IsDir:   false,
				Added:   tf.Added,
				Removed: tf.Removed,
			})
		}
	}

	flatten(root, 0, "")
	return nodes
}

func basename(path string) string {
	idx := strings.LastIndex(path, "/")
	if idx < 0 {
		return path
	}
	return path[idx+1:]
}

// treeFileNodes returns only the file (non-directory) nodes from TreeNodes.
func treeFileNodes(nodes []TreeNode) []int {
	var indices []int
	for i, n := range nodes {
		if !n.IsDir {
			indices = append(indices, i)
		}
	}
	return indices
}

// ClampTreeCursor ensures TreeCursor is within bounds of file nodes.
func (s *State) ClampTreeCursor() {
	fileIndices := treeFileNodes(s.TreeNodes)
	if len(fileIndices) == 0 {
		s.TreeCursor = 0
		return
	}
	if s.TreeCursor < 0 {
		s.TreeCursor = 0
	}
	if s.TreeCursor >= len(fileIndices) {
		s.TreeCursor = len(fileIndices) - 1
	}
}

// TreeCursorPath returns the file path at the current tree cursor position.
func (s *State) TreeCursorPath() string {
	fileIndices := treeFileNodes(s.TreeNodes)
	if len(fileIndices) == 0 {
		return ""
	}
	s.ClampTreeCursor()
	return s.TreeNodes[fileIndices[s.TreeCursor]].Path
}

// TreeCursorNodeIndex returns the TreeNodes index for the current cursor.
func (s *State) TreeCursorNodeIndex() int {
	fileIndices := treeFileNodes(s.TreeNodes)
	if len(fileIndices) == 0 {
		return -1
	}
	s.ClampTreeCursor()
	return fileIndices[s.TreeCursor]
}

// InitTreeCursorFromScroll sets the tree cursor to the file currently visible
// in the diff view (based on scroll position).
func (s *State) InitTreeCursorFromScroll() {
	currentFile := s.CurrentFile()
	if currentFile == "" {
		s.TreeCursor = 0
		return
	}
	fileIndices := treeFileNodes(s.TreeNodes)
	for ci, ni := range fileIndices {
		if s.TreeNodes[ni].Path == currentFile {
			s.TreeCursor = ci
			return
		}
	}
	s.TreeCursor = 0
}

// ClampTreeScroll ensures tree scroll is within valid bounds.
func (s *State) ClampTreeScroll() {
	maxVisible := s.Height - 3 // header + separator + status bar
	totalNodes := len(s.TreeNodes)
	if totalNodes <= maxVisible {
		s.TreeScroll = 0
		return
	}
	maxScroll := totalNodes - maxVisible
	if s.TreeScroll < 0 {
		s.TreeScroll = 0
	}
	if s.TreeScroll > maxScroll {
		s.TreeScroll = maxScroll
	}
}

// EnsureTreeCursorVisible scrolls the tree so the cursor is visible.
func (s *State) EnsureTreeCursorVisible() {
	nodeIdx := s.TreeCursorNodeIndex()
	if nodeIdx < 0 {
		return
	}
	maxVisible := s.Height - 3
	if maxVisible < 1 {
		maxVisible = 1
	}

	// Convert nodeIdx to visible row (relative to scroll)
	if nodeIdx < s.TreeScroll {
		s.TreeScroll = nodeIdx
	} else if nodeIdx >= s.TreeScroll+maxVisible {
		s.TreeScroll = nodeIdx - maxVisible + 1
	}
	s.ClampTreeScroll()
}

// drawTree renders the file tree sidebar
func drawTree(s *State) {
	screen := s.Screen
	tw := treeWidth

	// Determine current file (from diff scroll position) for highlighting
	currentFile := s.CurrentFile()

	// Styles for focused vs unfocused tree
	borderStyle := s.Theme.Dim
	if s.TreeFocused {
		borderStyle = tcell.StyleDefault.Foreground(s.Theme.Accent)
	}

	// Header
	header := fmt.Sprintf(" Files (%d)", len(s.TreeFiles))
	headerStyle := s.Theme.FileHeader
	if s.TreeFocused {
		headerStyle = tcell.StyleDefault.Bold(true).Foreground(s.Theme.Accent)
	}
	col := 0
	for _, r := range header {
		if col >= tw {
			break
		}
		screen.SetContent(col, 0, r, nil, headerStyle)
		col++
	}
	for col < tw {
		screen.SetContent(col, 0, ' ', nil, s.Theme.Default)
		col++
	}

	// Separator
	for x := 0; x < tw; x++ {
		screen.SetContent(x, 1, '─', nil, borderStyle)
	}

	// Tree nodes with scrolling
	s.ClampTreeScroll()
	maxVisible := s.Height - 3 // header + separator + status bar
	if maxVisible < 0 {
		maxVisible = 0
	}

	fileIndices := treeFileNodes(s.TreeNodes)
	cursorNodeIdx := -1
	if s.TreeFocused && len(fileIndices) > 0 {
		s.ClampTreeCursor()
		cursorNodeIdx = fileIndices[s.TreeCursor]
	}

	for i := 0; i < maxVisible; i++ {
		nodeIdx := s.TreeScroll + i
		y := i + 2
		if y >= s.Height-1 {
			break
		}

		if nodeIdx >= len(s.TreeNodes) {
			// Clear remaining rows
			for x := 0; x < tw; x++ {
				screen.SetContent(x, y, ' ', nil, s.Theme.Default)
			}
			continue
		}

		node := s.TreeNodes[nodeIdx]
		isCursorHere := nodeIdx == cursorNodeIdx
		isCurrentFile := !node.IsDir && node.Path == currentFile
		isFilteredFile := !node.IsDir && node.Path == s.FilterFile && s.FilterFile != ""

		drawTreeNode(s, screen, y, node, tw, isCursorHere, isCurrentFile, isFilteredFile, s.TreeFocused)
	}

	// Clear remaining rows if tree is shorter than visible area
	startClear := len(s.TreeNodes) - s.TreeScroll + 2
	if startClear < 2 {
		startClear = 2
	}
	for y := startClear; y < s.Height-1; y++ {
		for x := 0; x < tw; x++ {
			screen.SetContent(x, y, ' ', nil, s.Theme.Default)
		}
	}

	// Vertical divider
	for y := 0; y < s.Height-1; y++ {
		screen.SetContent(tw, y, '│', nil, borderStyle)
	}
}

func drawTreeNode(s *State, screen tcell.Screen, y int, node TreeNode, width int, isCursor, isActive, isFiltered, treeFocused bool) {
	col := 0

	// Background style for the entire row
	rowBg := s.Theme.Default
	if isCursor && treeFocused {
		rowBg = tcell.StyleDefault.Reverse(true)
	}

	// Indicator column
	indicator := ' '
	indicatorStyle := rowBg
	if isFiltered {
		indicator = '*'
		indicatorStyle = rowBg.Foreground(s.Theme.Highlight).Bold(true)
	} else if isActive && !node.IsDir {
		indicator = '▸'
		indicatorStyle = rowBg.Foreground(s.Theme.Highlight)
	}
	screen.SetContent(col, y, indicator, nil, indicatorStyle)
	col++

	// Indentation (2 spaces per depth level)
	indent := node.Depth * 2
	for i := 0; i < indent && col < width; i++ {
		screen.SetContent(col, y, ' ', nil, rowBg)
		col++
	}

	if node.IsDir {
		// Directory: show name with trailing /
		dirStyle := rowBg.Foreground(s.Theme.Accent)
		for _, r := range node.Display {
			if col >= width {
				break
			}
			screen.SetContent(col, y, r, nil, dirStyle)
			col++
		}
		// Fill rest
		for col < width {
			screen.SetContent(col, y, ' ', nil, rowBg)
			col++
		}
		return
	}

	// File leaf: name + stats
	addStr := fmt.Sprintf("+%d", node.Added)
	remStr := fmt.Sprintf("-%d", node.Removed)
	statsLen := len(addStr) + 1 + len(remStr)

	nameStyle := rowBg
	if isFiltered {
		nameStyle = rowBg.Foreground(s.Theme.Highlight).Bold(true)
	} else if isActive {
		nameStyle = rowBg.Bold(true)
	}

	maxName := width - statsLen - col - 1
	if maxName < 4 {
		maxName = 4
	}

	nameRunes := []rune(node.Display)
	if len(nameRunes) > maxName {
		nameRunes = append([]rune("…"), nameRunes[len(nameRunes)-maxName+1:]...)
	}
	for _, r := range nameRunes {
		if col >= width {
			break
		}
		screen.SetContent(col, y, r, nil, nameStyle)
		col++
	}

	// Pad to stats position
	statsStart := width - statsLen - 1
	for col < statsStart {
		screen.SetContent(col, y, ' ', nil, rowBg)
		col++
	}

	// Stats
	addStyle := rowBg.Foreground(s.Theme.Added)
	remStyle := rowBg.Foreground(s.Theme.Removed)
	for _, r := range addStr {
		if col >= width {
			break
		}
		screen.SetContent(col, y, r, nil, addStyle)
		col++
	}
	if col < width {
		screen.SetContent(col, y, ' ', nil, rowBg)
		col++
	}
	for _, r := range remStr {
		if col >= width {
			break
		}
		screen.SetContent(col, y, r, nil, remStyle)
		col++
	}

	for col < width {
		screen.SetContent(col, y, ' ', nil, rowBg)
		col++
	}
}
