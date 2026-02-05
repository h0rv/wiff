package main

import "testing"

func TestReservedKeysContainsKnownBindings(t *testing.T) {
	known := []rune{'q', 'j', 'k', 'd', 'u', 's', 'w', 'e', 'h', 'b', 'o', '/', '?', 'f', 'W'}
	for _, r := range known {
		if !reservedKeys[r] {
			t.Errorf("expected '%c' to be reserved", r)
		}
	}
}

func TestReservedKeysExcludesUnbound(t *testing.T) {
	unbound := []rune{'z', 'x', 'Z', 'X'}
	for _, r := range unbound {
		if reservedKeys[r] {
			t.Errorf("expected '%c' to NOT be reserved", r)
		}
	}
}

func TestAvailableLabelsNonEmpty(t *testing.T) {
	if len(availableLabels) == 0 {
		t.Fatal("availableLabels must not be empty")
	}
}

func TestAvailableLabelsExcludesReserved(t *testing.T) {
	for _, r := range availableLabels {
		if reservedKeys[r] {
			t.Errorf("label '%c' should not be in reservedKeys", r)
		}
	}
}

func TestAvailableLabelsStartsWithLowercase(t *testing.T) {
	first := availableLabels[0]
	if first < 'a' || first > 'z' {
		t.Errorf("expected first available label to be lowercase, got '%c'", first)
	}
}

func TestIndexToLabelSingleChar(t *testing.T) {
	for i := 0; i < len(availableLabels); i++ {
		label := indexToLabel(i)
		if len(label) != 1 {
			t.Errorf("indexToLabel(%d) = %q, expected single char", i, label)
		}
	}
}

func TestIndexToLabelTwoChar(t *testing.T) {
	label := indexToLabel(len(availableLabels))
	if len(label) != 2 {
		t.Errorf("indexToLabel(%d) = %q, expected two chars", len(availableLabels), label)
	}
}

func TestIndexToLabelConsistency(t *testing.T) {
	// Same index should always produce the same label
	for i := 0; i < 10; i++ {
		a := indexToLabel(i)
		b := indexToLabel(i)
		if a != b {
			t.Errorf("indexToLabel(%d) inconsistent: %q vs %q", i, a, b)
		}
	}
}
