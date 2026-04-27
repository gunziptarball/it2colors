package main

import (
	"bytes"
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lucasb-eyer/go-colorful"
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

func TestRotateHue(t *testing.T) {
	red := Color{Red: 0.8, Green: 0.2, Blue: 0.2}
	gray := Color{Red: 0.5, Green: 0.5, Blue: 0.5}
	black := Color{Red: 0, Green: 0, Blue: 0}
	in := Profile{
		"Foreground Color": red,
		"Background Color": gray,
		"Bold Color":       black,
	}

	t.Run("zero is identity", func(t *testing.T) {
		out := rotateHue(in, 0)
		// Same map returned — no allocation, no transform.
		if &out == nil || out["Foreground Color"] != red {
			t.Errorf("rotateHue(_, 0) should be identity")
		}
	})

	t.Run("360 returns to same hue", func(t *testing.T) {
		out := rotateHue(in, 360)
		got := out["Foreground Color"]
		if !approxColor(got, red, 0.01) {
			t.Errorf("rotateHue(red, 360) = %+v, want approx %+v", got, red)
		}
	})

	t.Run("preserves lightness", func(t *testing.T) {
		_, _, lIn := (colorful.Color{R: red.Red, G: red.Green, B: red.Blue}).Hcl()
		got := rotateHue(in, 120)["Foreground Color"]
		_, _, lOut := (colorful.Color{R: got.Red, G: got.Green, B: got.Blue}).Hcl()
		if math.Abs(lIn-lOut) > 0.05 {
			t.Errorf("lightness drifted: in=%v out=%v", lIn, lOut)
		}
	})

	t.Run("achromatic colors pass through", func(t *testing.T) {
		out := rotateHue(in, 90)
		if out["Background Color"] != gray {
			t.Errorf("gray should not rotate, got %+v", out["Background Color"])
		}
		if out["Bold Color"] != black {
			t.Errorf("black should not rotate, got %+v", out["Bold Color"])
		}
	})

	t.Run("rotates hue by the requested amount", func(t *testing.T) {
		hIn, _, _ := (colorful.Color{R: red.Red, G: red.Green, B: red.Blue}).Hcl()
		got := rotateHue(in, 90)["Foreground Color"]
		hOut, _, _ := (colorful.Color{R: got.Red, G: got.Green, B: got.Blue}).Hcl()
		want := math.Mod(hIn+90, 360)
		// Generous tolerance: gamut clamping can shift hue several degrees
		// when a saturated rotation lands outside displayable sRGB.
		if math.Abs(hOut-want) > 8.0 {
			t.Errorf("hue: in=%v +90 → got %v, want ≈ %v", hIn, hOut, want)
		}
	})
}

func TestRotateHueChangesRealScheme(t *testing.T) {
	p, err := loadProfile(filepath.Join("testdata", "Adventure.itermcolors"))
	if err != nil {
		t.Fatal(err)
	}
	var orig, rot bytes.Buffer
	if err := applyProfile(&orig, p); err != nil {
		t.Fatal(err)
	}
	if err := applyProfile(&rot, rotateHue(p, 45)); err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(orig.Bytes(), rot.Bytes()) {
		t.Fatal("rotateHue(_, 45) produced identical OSC bytes — rotation is a no-op")
	}
	t.Logf("orig and rotated differ as expected (orig=%dB, rot=%dB)", orig.Len(), rot.Len())
}

func approxColor(a, b Color, eps float64) bool {
	return math.Abs(a.Red-b.Red) < eps &&
		math.Abs(a.Green-b.Green) < eps &&
		math.Abs(a.Blue-b.Blue) < eps
}

func TestPrintPreview(t *testing.T) {
	var buf bytes.Buffer
	if err := printPreview(&buf); err != nil {
		t.Fatalf("printPreview: %v", err)
	}
	got := buf.String()

	for _, want := range []string{
		"def     40m     41m",                  // header
		"\x1b[31m  gYw  ",                      // red fg on default bg
		"\x1b[1;31m\x1b[44m  gYw  \x1b[0m",     // bold red on blue, with reset
	} {
		if !strings.Contains(got, want) {
			t.Errorf("printPreview output missing %q", want)
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
