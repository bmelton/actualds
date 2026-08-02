package main

import (
	"slices"
	"testing"
)

func TestHandleKey(t *testing.T) {
	up := []byte{0x1b, '[', 'A'}
	down := []byte{0x1b, '[', 'B'}

	cases := []struct {
		name    string
		key     []byte
		cur     int
		on      []bool
		wantCur int
		wantOn  []bool
		wantAct action
	}{
		{"down moves", down, 0, []bool{true, true, true}, 1, []bool{true, true, true}, actNone},
		{"down wraps", down, 2, []bool{true, true, true}, 0, []bool{true, true, true}, actNone},
		{"up wraps", up, 0, []bool{true, true, true}, 2, []bool{true, true, true}, actNone},
		{"j moves", []byte{'j'}, 0, []bool{true, true}, 1, []bool{true, true}, actNone},
		{"k wraps", []byte{'k'}, 0, []bool{true, true}, 1, []bool{true, true}, actNone},
		{"space toggles off", []byte{' '}, 1, []bool{true, true}, 1, []bool{true, false}, actNone},
		{"space toggles on", []byte{' '}, 0, []bool{false, true}, 0, []bool{true, true}, actNone},
		{"a selects all", []byte{'a'}, 0, []bool{false, false}, 0, []bool{true, true}, actNone},
		{"n clears all", []byte{'n'}, 0, []bool{true, true}, 0, []bool{false, false}, actNone},
		{"enter confirms", []byte{'\r'}, 0, []bool{true}, 0, []bool{true}, actConfirm},
		{"newline confirms", []byte{'\n'}, 0, []bool{true}, 0, []bool{true}, actConfirm},
		{"q quits", []byte{'q'}, 0, []bool{true}, 0, []bool{true}, actQuit},
		{"ctrl-c quits", []byte{0x03}, 0, []bool{true}, 0, []bool{true}, actQuit},
		{"esc quits", []byte{0x1b}, 0, []bool{true}, 0, []bool{true}, actQuit},
		// pick treats a zero-byte read as EOF before calling this.
		{"empty buffer is a no-op", []byte{}, 0, []bool{true}, 0, []bool{true}, actNone},
		{"unknown key ignored", []byte{'z'}, 0, []bool{true}, 0, []bool{true}, actNone},
	}

	for _, c := range cases {
		gotCur, gotAct := handleKeys(c.key, c.cur, c.on)
		if gotCur != c.wantCur {
			t.Errorf("%s: cursor = %d, want %d", c.name, gotCur, c.wantCur)
		}
		if gotAct != c.wantAct {
			t.Errorf("%s: action = %v, want %v", c.name, gotAct, c.wantAct)
		}
		if !slices.Equal(c.on, c.wantOn) {
			t.Errorf("%s: selection = %v, want %v", c.name, c.on, c.wantOn)
		}
	}
}

// An empty list must not divide by zero when computing the wrapped cursor.
func TestHandleKeyEmptyList(t *testing.T) {
	if _, act := handleKeys([]byte{'j'}, 0, nil); act != actQuit {
		t.Errorf("empty list: action = %v, want actQuit", act)
	}
}

// One read can deliver several keystrokes at once. Every one has to be
// applied; an earlier version consumed only the first and hung because the
// trailing quit key was dropped.
func TestHandleKeysDrainsWholeBuffer(t *testing.T) {
	on := []bool{true, true, true, true}
	// down, down, space, q
	cur, act := handleKeys([]byte{0x1b, '[', 'B', 0x1b, '[', 'B', ' ', 'q'}, 0, on)
	if cur != 2 {
		t.Errorf("cursor = %d, want 2", cur)
	}
	if act != actQuit {
		t.Errorf("action = %v, want actQuit", act)
	}
	if want := []bool{true, true, false, true}; !slices.Equal(on, want) {
		t.Errorf("selection = %v, want %v", on, want)
	}
}

// Keys after a confirm must not be applied, or a stray byte could toggle
// something after the user already committed.
func TestHandleKeysStopsAtConfirm(t *testing.T) {
	on := []bool{true, true}
	_, act := handleKeys([]byte{'\r', 'n'}, 0, on)
	if act != actConfirm {
		t.Errorf("action = %v, want actConfirm", act)
	}
	if want := []bool{true, true}; !slices.Equal(on, want) {
		t.Errorf("selection = %v, want %v (keys after confirm were applied)", on, want)
	}
}

func TestClip(t *testing.T) {
	if got := clip("abcdefgh", 5); got != "abcd" {
		t.Errorf("clip truncate = %q, want %q", got, "abcd")
	}
	if got := clip("abc", 80); got != "abc" {
		t.Errorf("clip passthrough = %q, want %q", got, "abc")
	}
}
