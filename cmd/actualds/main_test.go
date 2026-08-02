package main

import "testing"

func TestKeep(t *testing.T) {
	cases := []struct {
		from, on string
		flags    uint32
		want     bool
	}{
		{"/dev/disk3s1s1", "/", mntRdonly, false},
		{"/dev/disk4s2", "/Volumes/Backup", 0, true},
		{"/dev/disk5s1", "/Volumes/SomeISO", mntRdonly, false},
		{"/dev/disk3s5", "/System/Volumes/Data", 0, true},
		{"/dev/disk3s6", "/System/Volumes/VM", 0, false},
		{"devfs", "/dev", 0, false},
		{"map auto_home", "/System/Volumes/Data/home", 0, false},
	}
	for _, c := range cases {
		if got := keep(c.from, c.on, c.flags); got != c.want {
			t.Errorf("keep(%q, %q, %#x) = %v, want %v", c.from, c.on, c.flags, got, c.want)
		}
	}
}
