package main

import "testing"

func TestBuildTreeNodesEmpty(t *testing.T) {
	nodes := buildTreeNodes(nil)
	if nodes != nil {
		t.Errorf("expected nil for empty input, got %d nodes", len(nodes))
	}
}

func TestBuildTreeNodesFlat(t *testing.T) {
	files := []TreeFile{
		{Path: "a.go", Added: 1, Removed: 2},
		{Path: "b.go", Added: 3, Removed: 0},
	}
	nodes := buildTreeNodes(files)

	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}
	for _, n := range nodes {
		if n.IsDir {
			t.Errorf("expected file node, got dir: %s", n.Display)
		}
		if n.Depth != 0 {
			t.Errorf("expected depth 0, got %d for %s", n.Depth, n.Display)
		}
	}
}

func TestBuildTreeNodesNested(t *testing.T) {
	files := []TreeFile{
		{Path: "src/pkg/a.go"},
		{Path: "src/pkg/b.go"},
		{Path: "README.md"},
	}
	nodes := buildTreeNodes(files)

	// Should have: collapsed dir "src/pkg/", two files under it, and "README.md" at root
	var dirs, fileNodes int
	for _, n := range nodes {
		if n.IsDir {
			dirs++
		} else {
			fileNodes++
		}
	}
	if fileNodes != 3 {
		t.Errorf("expected 3 file nodes, got %d", fileNodes)
	}
	if dirs != 1 {
		t.Errorf("expected 1 collapsed dir node, got %d", dirs)
	}
}

func TestBuildTreeNodesCollapsing(t *testing.T) {
	files := []TreeFile{
		{Path: "a/b/c/file.go"},
	}
	nodes := buildTreeNodes(files)

	// Should have: one collapsed dir "a/b/c/" and one file
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes (dir + file), got %d", len(nodes))
	}
	if !nodes[0].IsDir {
		t.Error("expected first node to be a directory")
	}
	if nodes[0].Display != "a/b/c/" {
		t.Errorf("expected collapsed dir 'a/b/c/', got '%s'", nodes[0].Display)
	}
	if nodes[1].IsDir {
		t.Error("expected second node to be a file")
	}
}

func TestBuildTreeNodesStats(t *testing.T) {
	files := []TreeFile{
		{Path: "x.go", Added: 5, Removed: 3},
	}
	nodes := buildTreeNodes(files)

	var fileNode *TreeNode
	for i := range nodes {
		if !nodes[i].IsDir {
			fileNode = &nodes[i]
			break
		}
	}
	if fileNode == nil {
		t.Fatal("expected a file node")
	}
	if fileNode.Added != 5 {
		t.Errorf("expected Added=5, got %d", fileNode.Added)
	}
	if fileNode.Removed != 3 {
		t.Errorf("expected Removed=3, got %d", fileNode.Removed)
	}
}

func TestTreeFileNodes(t *testing.T) {
	nodes := []TreeNode{
		{Display: "src/", IsDir: true},
		{Display: "a.go", IsDir: false},
		{Display: "pkg/", IsDir: true},
		{Display: "b.go", IsDir: false},
	}

	indices := treeFileNodes(nodes)
	if len(indices) != 2 {
		t.Fatalf("expected 2 file indices, got %d", len(indices))
	}
	if indices[0] != 1 {
		t.Errorf("expected first file index 1, got %d", indices[0])
	}
	if indices[1] != 3 {
		t.Errorf("expected second file index 3, got %d", indices[1])
	}
}

func TestClampTreeCursor(t *testing.T) {
	s := &State{
		TreeNodes: []TreeNode{
			{Display: "dir/", IsDir: true},
			{Display: "a.go", IsDir: false, Path: "a.go"},
			{Display: "b.go", IsDir: false, Path: "b.go"},
			{Display: "c.go", IsDir: false, Path: "c.go"},
		},
	}

	s.TreeCursor = -1
	s.ClampTreeCursor()
	if s.TreeCursor != 0 {
		t.Errorf("expected cursor clamped to 0, got %d", s.TreeCursor)
	}

	s.TreeCursor = 100
	s.ClampTreeCursor()
	if s.TreeCursor != 2 { // 3 file nodes, max index = 2
		t.Errorf("expected cursor clamped to 2, got %d", s.TreeCursor)
	}
}

func TestClampTreeCursorEmpty(t *testing.T) {
	s := &State{TreeNodes: nil}
	s.TreeCursor = 5
	s.ClampTreeCursor()
	if s.TreeCursor != 0 {
		t.Errorf("expected cursor 0 for empty nodes, got %d", s.TreeCursor)
	}
}

func TestTreeCursorPath(t *testing.T) {
	s := &State{
		TreeNodes: []TreeNode{
			{Display: "dir/", IsDir: true},
			{Display: "a.go", IsDir: false, Path: "dir/a.go"},
			{Display: "b.go", IsDir: false, Path: "dir/b.go"},
		},
	}

	s.TreeCursor = 0
	if got := s.TreeCursorPath(); got != "dir/a.go" {
		t.Errorf("expected 'dir/a.go', got '%s'", got)
	}

	s.TreeCursor = 1
	if got := s.TreeCursorPath(); got != "dir/b.go" {
		t.Errorf("expected 'dir/b.go', got '%s'", got)
	}
}

func TestTreeCursorPathEmpty(t *testing.T) {
	s := &State{TreeNodes: nil}
	if got := s.TreeCursorPath(); got != "" {
		t.Errorf("expected empty string, got '%s'", got)
	}
}

func TestEnsureTreeCursorVisible(t *testing.T) {
	// Create 20 nodes (all files for simplicity)
	nodes := make([]TreeNode, 20)
	for i := range nodes {
		nodes[i] = TreeNode{Display: "f.go", IsDir: false, Path: "f.go"}
	}

	s := &State{
		Height:     12, // 12 - 3 = 9 visible
		TreeNodes:  nodes,
		TreeScroll: 0,
		TreeCursor: 15,
	}

	s.EnsureTreeCursorVisible()
	// Node at index 15 should now be visible. TreeScroll should have moved.
	maxVisible := s.Height - 3
	if s.TreeScroll+maxVisible <= 15 {
		t.Errorf("expected tree to scroll so index 15 is visible, TreeScroll=%d maxVisible=%d",
			s.TreeScroll, maxVisible)
	}
}

func TestBasename(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"a/b/c.go", "c.go"},
		{"file.go", "file.go"},
		{"a/b/", ""},
		{"x", "x"},
	}
	for _, tt := range tests {
		if got := basename(tt.in); got != tt.want {
			t.Errorf("basename(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
