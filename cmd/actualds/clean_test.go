package main

import "testing"

func TestParseSize(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"21.25GB", 21_250_000_000},
		{"2.781GB", 2_781_000_000},
		{"0B", 0},
		{"512kB", 512_000},
		{"1.5TB", 1_500_000_000_000},
		{"75.24MB", 75_240_000},
		{"garbage", 0},
	}
	for _, c := range cases {
		if got := parseSize(c.in); got != c.want {
			t.Errorf("parseSize(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}
