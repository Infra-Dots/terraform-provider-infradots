package internal

import "testing"

func TestNormalizeCrontab(t *testing.T) {
	cases := map[string]string{
		// The reported bug: the API annotates the crontab it echoes back.
		"0 9 * * 1 (m/h/dM/MY/d) UTC":   "0 9 * * 1",
		"*/5 * * * * (m/h/dM/MY/d) UTC": "*/5 * * * *",
		// Already-plain expressions are returned unchanged.
		"0 12 * * *": "0 12 * * *",
		// Surrounding whitespace is trimmed.
		"  0 9 * * 1  ": "0 9 * * 1",
		"":              "",
	}
	for in, want := range cases {
		if got := normalizeCrontab(in); got != want {
			t.Errorf("normalizeCrontab(%q) = %q, want %q", in, got, want)
		}
	}
}
