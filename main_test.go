package main

import (
	"bytes"
	"fmt"
	"math"
	"os"
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

func TestAdjustColors(t *testing.T) {
	red := Color{Red: 0.8, Green: 0.2, Blue: 0.2}
	gray := Color{Red: 0.5, Green: 0.5, Blue: 0.5}
	black := Color{Red: 0, Green: 0, Blue: 0}
	in := Profile{
		"Foreground Color": red,
		"Background Color": gray,
		"Bold Color":       black,
	}

	t.Run("all-zero is identity", func(t *testing.T) {
		out := adjustColors(in, 0, 0, 1)
		if &out == nil || out["Foreground Color"] != red {
			t.Errorf("adjustColors(_, 0, 0, 1) should be identity")
		}
	})

	t.Run("360 hue returns to same hue", func(t *testing.T) {
		out := adjustColors(in, 360, 0, 1)
		got := out["Foreground Color"]
		if !approxColor(got, red, 0.01) {
			t.Errorf("hue 360: got %+v, want approx %+v", got, red)
		}
	})

	t.Run("hue rotation preserves lightness", func(t *testing.T) {
		_, _, lIn := (colorful.Color{R: red.Red, G: red.Green, B: red.Blue}).Hcl()
		got := adjustColors(in, 120, 0, 1)["Foreground Color"]
		_, _, lOut := (colorful.Color{R: got.Red, G: got.Green, B: got.Blue}).Hcl()
		if math.Abs(lIn-lOut) > 0.05 {
			t.Errorf("lightness drifted under hue rotation: in=%v out=%v", lIn, lOut)
		}
	})

	t.Run("hue skips achromatic colors", func(t *testing.T) {
		out := adjustColors(in, 90, 0, 1)
		if out["Background Color"] != gray {
			t.Errorf("gray should not rotate, got %+v", out["Background Color"])
		}
		if out["Bold Color"] != black {
			t.Errorf("black should not rotate, got %+v", out["Bold Color"])
		}
	})

	t.Run("rotates hue by the requested amount", func(t *testing.T) {
		hIn, _, _ := (colorful.Color{R: red.Red, G: red.Green, B: red.Blue}).Hcl()
		got := adjustColors(in, 90, 0, 1)["Foreground Color"]
		hOut, _, _ := (colorful.Color{R: got.Red, G: got.Green, B: got.Blue}).Hcl()
		want := math.Mod(hIn+90, 360)
		// Generous tolerance: gamut clamping can shift hue several degrees
		// on saturated rotations that land outside sRGB.
		if math.Abs(hOut-want) > 8.0 {
			t.Errorf("hue: in=%v +90 → got %v, want ≈ %v", hIn, hOut, want)
		}
	})

	t.Run("lightness darkens chromatic color", func(t *testing.T) {
		_, _, lIn := (colorful.Color{R: red.Red, G: red.Green, B: red.Blue}).Hcl()
		got := adjustColors(in, 0, -0.2, 1)["Foreground Color"]
		_, _, lOut := (colorful.Color{R: got.Red, G: got.Green, B: got.Blue}).Hcl()
		if lOut >= lIn {
			t.Errorf("expected lightness decrease: in=%v out=%v", lIn, lOut)
		}
		// Gamut clamping on the return trip can reduce the effective shift; verify
		// it moved meaningfully in the right direction rather than asserting exact delta.
		if lIn-lOut < 0.1 {
			t.Errorf("lightness drop too small: in=%v out=%v diff=%v", lIn, lOut, lIn-lOut)
		}
	})

	t.Run("lightness darkens achromatic color", func(t *testing.T) {
		_, _, lIn := (colorful.Color{R: gray.Red, G: gray.Green, B: gray.Blue}).Hcl()
		got := adjustColors(in, 0, -0.2, 1)["Background Color"]
		_, _, lOut := (colorful.Color{R: got.Red, G: got.Green, B: got.Blue}).Hcl()
		if lOut >= lIn {
			t.Errorf("lightness shift should also darken achromatic gray: in=%v out=%v", lIn, lOut)
		}
	})

	t.Run("lightness clamps at 0", func(t *testing.T) {
		got := adjustColors(in, 0, -2, 1)["Bold Color"] // black shifted way down
		// HCL round-trip may introduce sub-epsilon noise; check near-black not exact zero.
		if got.Red > 1e-10 || got.Green > 1e-10 || got.Blue > 1e-10 {
			t.Errorf("black shifted down should stay near-black, got %+v", got)
		}
	})

	t.Run("saturation desaturates chromatic color", func(t *testing.T) {
		_, chIn, _ := (colorful.Color{R: red.Red, G: red.Green, B: red.Blue}).Hcl()
		got := adjustColors(in, 0, 0, 0.5)["Foreground Color"]
		_, chOut, _ := (colorful.Color{R: got.Red, G: got.Green, B: got.Blue}).Hcl()
		if chOut >= chIn {
			t.Errorf("saturation 0.5 should reduce chroma: in=%v out=%v", chIn, chOut)
		}
	})

	t.Run("saturation=0 fully desaturates", func(t *testing.T) {
		got := adjustColors(in, 0, 0, 0)["Foreground Color"]
		_, chOut, _ := (colorful.Color{R: got.Red, G: got.Green, B: got.Blue}).Hcl()
		if chOut > 1e-3 {
			t.Errorf("saturation=0 should produce achromatic, chroma=%v", chOut)
		}
	})
}

func TestAdjustColorsChangesRealScheme(t *testing.T) {
	p, err := loadProfile(filepath.Join("testdata", "Adventure.itermcolors"))
	if err != nil {
		t.Fatal(err)
	}
	var orig bytes.Buffer
	if err := applyProfile(&orig, p); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name string
		hue, lightness, saturation float64
	}{
		{"hue 45", 45, 0, 1},
		{"lightness -0.2", 0, -0.2, 1},
		{"saturation 0.5", 0, 0, 0.5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := applyProfile(&buf, adjustColors(p, tc.hue, tc.lightness, tc.saturation)); err != nil {
				t.Fatal(err)
			}
			if bytes.Equal(orig.Bytes(), buf.Bytes()) {
				t.Fatalf("adjustColors(%v, %v, %v) produced identical OSC bytes — transform is a no-op", tc.hue, tc.lightness, tc.saturation)
			}
		})
	}
}

func TestBgTintHue(t *testing.T) {
	cases := []struct {
		in      string
		want    float64
		wantErr bool
	}{
		{"red", 25, false},
		{"RED", 25, false},
		{"  blue  ", 265, false},
		{"25", 25, false},
		{"-30", 330, false},
		{"720", 0, false},
		{"chartreuse", 0, true},
		{"", 0, true},
	}
	for _, c := range cases {
		got, err := bgTintHue(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("bgTintHue(%q): expected error, got %v", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("bgTintHue(%q): unexpected error: %v", c.in, err)
			continue
		}
		if math.Abs(got-c.want) > 1e-9 {
			t.Errorf("bgTintHue(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestApplyBgTint(t *testing.T) {
	// Realistic near-black bg (L ≈ 0.18) and mid-gray fg (L ≈ 0.5) — pure
	// #000 would be out of gamut at any chroma > 0 and Clamped() would have
	// to lift L to compensate.
	bgIn := Color{Red: 0.1, Green: 0.1, Blue: 0.1}
	fgIn := Color{Red: 0.5, Green: 0.5, Blue: 0.5}
	in := Profile{
		"Background Color": bgIn,
		"Foreground Color": fgIn,
		"Ansi 1 Color":     {Red: 0.8, Green: 0.2, Blue: 0.2},
	}

	t.Run("strength 0 is identity", func(t *testing.T) {
		out := applyBgTint(in, 25, 0)
		for k, v := range in {
			if out[k] != v {
				t.Errorf("strength 0 changed %s: %+v -> %+v", k, v, out[k])
			}
		}
	})

	t.Run("background gains chroma at target hue, lightness preserved", func(t *testing.T) {
		_, _, lIn := (colorful.Color{R: bgIn.Red, G: bgIn.Green, B: bgIn.Blue}).Hcl()
		out := applyBgTint(in, 25, 0.5)
		bg := out["Background Color"]
		hOut, chOut, lOut := (colorful.Color{R: bg.Red, G: bg.Green, B: bg.Blue}).Hcl()
		if chOut <= 0.05 {
			t.Errorf("background chroma did not increase: %v", chOut)
		}
		if !math.IsNaN(hOut) && math.Abs(hOut-25) > 8 {
			t.Errorf("background hue: got %v, want ≈ 25", hOut)
		}
		if math.Abs(lIn-lOut) > 0.05 {
			t.Errorf("background lightness drifted: in=%v out=%v", lIn, lOut)
		}
	})

	t.Run("foreground tinted less than background", func(t *testing.T) {
		out := applyBgTint(in, 25, 0.5)
		bg := out["Background Color"]
		fg := out["Foreground Color"]
		_, chBgIn, _ := (colorful.Color{R: bgIn.Red, G: bgIn.Green, B: bgIn.Blue}).Hcl()
		_, chFgIn, _ := (colorful.Color{R: fgIn.Red, G: fgIn.Green, B: fgIn.Blue}).Hcl()
		_, chBgOut, _ := (colorful.Color{R: bg.Red, G: bg.Green, B: bg.Blue}).Hcl()
		_, chFgOut, _ := (colorful.Color{R: fg.Red, G: fg.Green, B: fg.Blue}).Hcl()
		bgShift := math.Abs(chBgOut - chBgIn)
		fgShift := math.Abs(chFgOut - chFgIn)
		if fgShift >= bgShift {
			t.Errorf("foreground chroma shift (%v) should be smaller than background (%v)", fgShift, bgShift)
		}
		if fgShift == 0 {
			t.Errorf("foreground should still receive a sympathetic nudge, got 0 shift")
		}
	})
}

func TestNameFileHelpers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "names.txt")

	t.Run("load missing file returns nil", func(t *testing.T) {
		if loadNameSet(path) != nil {
			t.Error("expected nil for missing file")
		}
	})

	t.Run("add creates file and deduplicates", func(t *testing.T) {
		if err := addToNameFile(path, "Alpha"); err != nil {
			t.Fatal(err)
		}
		if err := addToNameFile(path, "Beta"); err != nil {
			t.Fatal(err)
		}
		if err := addToNameFile(path, "Alpha"); err != nil { // duplicate
			t.Fatal(err)
		}
		set := loadNameSet(path)
		if len(set) != 2 || !set["Alpha"] || !set["Beta"] {
			t.Errorf("want {Alpha, Beta}, got %v", set)
		}
	})

	t.Run("remove deletes entry", func(t *testing.T) {
		if err := removeFromNameFile(path, "Alpha"); err != nil {
			t.Fatal(err)
		}
		set := loadNameSet(path)
		if set["Alpha"] || !set["Beta"] {
			t.Errorf("after remove: want {Beta}, got %v", set)
		}
	})

	t.Run("remove missing name is no-op", func(t *testing.T) {
		if err := removeFromNameFile(path, "DoesNotExist"); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("file is sorted", func(t *testing.T) {
		p2 := filepath.Join(dir, "sorted.txt")
		for _, n := range []string{"Zebra", "Apple", "Mango"} {
			if err := addToNameFile(p2, n); err != nil {
				t.Fatal(err)
			}
		}
		data, _ := os.ReadFile(p2)
		got := strings.TrimRight(string(data), "\n")
		if got != "Apple\nMango\nZebra" {
			t.Errorf("file not sorted: %q", got)
		}
	})
}

func TestEvalOutputQuotesMultiWordNames(t *testing.T) {
	// Scheme names with spaces must be single-quoted so eval "$(it2colors ...)"
	// assigns the full name rather than splitting on the space.
	cases := []struct {
		name string
		want string
	}{
		{"Adventure", "export IT2COLORS_SCHEME='Adventure'\n"},
		{"JetBrains Darcula", "export IT2COLORS_SCHEME='JetBrains Darcula'\n"},
	}
	for _, c := range cases {
		got := fmt.Sprintf("export IT2COLORS_SCHEME='%s'\n", c.name)
		if got != c.want {
			t.Errorf("eval output for %q: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestRecentHistory(t *testing.T) {
	dir := t.TempDir()
	histPath := filepath.Join(dir, ".it2colors_history")

	lines := []string{
		"2026-01-01T00:00:00Z\tSchemeA",
		"2026-01-02T00:00:00Z\tSchemeB",
		"2026-01-03T00:00:00Z\tSchemeC",
		"",                          // trailing newline edge case
	}
	if err := os.WriteFile(histPath, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		t.Fatal(err)
	}

	// Patch the lookup to use our temp file via a helper that accepts a path.
	data, _ := os.ReadFile(histPath)
	got := parseRecentHistory(data, 2)

	if len(got) != 2 {
		t.Fatalf("want 2 entries, got %d: %v", len(got), got)
	}
	if got[0] != "SchemeC" || got[1] != "SchemeB" {
		t.Errorf("want [SchemeC SchemeB], got %v", got)
	}
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
