package main

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestParseProfile(t *testing.T) {
	p, err := loadProfile(filepath.Join("testdata", "Adventure.itermcolors"))
	if err != nil {
		t.Fatalf("loadProfile: %v", err)
	}

	fg, ok := p["Foreground Color"]
	if !ok {
		t.Fatal("Foreground Color missing")
	}
	if fg.Red != 0.9960784316062927 || fg.Green != 1 || fg.Blue != 1 {
		t.Errorf("Foreground Color = %+v, want R=0.9960784316062927 G=1 B=1", fg)
	}

	a0, ok := p["Ansi 0 Color"]
	if !ok {
		t.Fatal("Ansi 0 Color missing")
	}
	const want = 0.015686275437474251
	if a0.Red != want || a0.Green != want || a0.Blue != want {
		t.Errorf("Ansi 0 Color = %+v, want all %v", a0, want)
	}
}

func TestRenderEscapes(t *testing.T) {
	p := Profile{
		"Foreground Color": {Red: 1, Green: 1, Blue: 1},
		"Ansi 0 Color":     {Red: 0.015686275437474251, Green: 0.015686275437474251, Blue: 0.015686275437474251},
		"Ansi 10 Color":    {Red: 0, Green: 1, Blue: 0},
		"Tab Color":        {Red: 0.5, Green: 0.5, Blue: 0.5}, // no slot, must be skipped
	}

	var buf bytes.Buffer
	if err := applyProfile(&buf, p); err != nil {
		t.Fatalf("applyProfile: %v", err)
	}

	got := buf.String()

	// Keys are written in sorted order by applyProfile.
	// Sorted: "Ansi 0 Color", "Ansi 10 Color", "Foreground Color", "Tab Color"
	want := "\x1b]P0040404\x1b\\" + // Ansi 0  → slot '0', 0.0156*256=4 → 04
		"\x1b]Pa00ff00\x1b\\" + //   Ansi 10 → slot 'a', 0,255,0 → 00ff00
		"\x1b]Pgffffff\x1b\\" //     Foreground → slot 'g', 1.0 clamps to ff
	if got != want {
		t.Errorf("escapes mismatch\n got: %q\nwant: %q", got, want)
	}
}

func TestSlotFor(t *testing.T) {
	cases := []struct {
		key  string
		slot byte
		ok   bool
	}{
		{"Foreground Color", 'g', true},
		{"Background Color", 'h', true},
		{"Cursor Text Color", 'm', true},
		{"Ansi 0 Color", '0', true},
		{"Ansi 9 Color", '9', true},
		{"Ansi 10 Color", 'a', true},
		{"Ansi 15 Color", 'f', true},
		{"Tab Color", 0, false},
		{"Underline Color", 0, false},
		{"Ansi 16 Color", 0, false},
	}
	for _, c := range cases {
		got, ok := slotFor(c.key)
		if got != c.slot || ok != c.ok {
			t.Errorf("slotFor(%q) = (%q, %v), want (%q, %v)", c.key, got, ok, c.slot, c.ok)
		}
	}
}

func TestClampByte(t *testing.T) {
	cases := []struct {
		in   float64
		want int
	}{
		{0, 0},
		{1, 255},   // 1.0*256 = 256 → clamped to 255
		{0.5, 128}, // 0.5*256 = 128
		{2.0, 255}, // overflow
		{-1, 0},    // underflow
	}
	for _, c := range cases {
		if got := clampByte(c.in); got != c.want {
			t.Errorf("clampByte(%v) = %d, want %d", c.in, got, c.want)
		}
	}
}
