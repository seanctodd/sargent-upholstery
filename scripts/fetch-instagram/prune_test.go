package main

import "testing"

func TestOursOnlyMatchesFilesThisScriptWrites(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		// What the script writes.
		{"18428911273135618.jpg", true},
		{".tmp-123456", true}, // interrupted download

		// What it must never delete.
		{".gitkeep", false},
		{"README.md", false},
		{"logo.svg", false},
		{"poster.png", false},
		{"notes.txt", false},
		{"", false},
	}
	for _, c := range cases {
		if got := ours(c.name); got != c.want {
			t.Errorf("ours(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}
