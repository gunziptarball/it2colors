package main

import (
	"encoding/json"
	"testing"
)

func TestAdjustmentUnmarshalDefaultsSaturation(t *testing.T) {
	var a Adjustment
	if err := json.Unmarshal([]byte(`{"hue": 30}`), &a); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if a.Hue != 30 {
		t.Errorf("hue = %v, want 30", a.Hue)
	}
	if a.Saturation != 1.0 {
		t.Errorf("saturation = %v, want 1.0 (default for missing field)", a.Saturation)
	}
}

func TestAdjustmentUnmarshalExplicitZeroSaturation(t *testing.T) {
	var a Adjustment
	if err := json.Unmarshal([]byte(`{"saturation": 0}`), &a); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if a.Saturation != 0 {
		t.Errorf("saturation = %v, want 0 (explicit zero preserved)", a.Saturation)
	}
}

func TestAdjustmentIsNoOp(t *testing.T) {
	cases := []struct {
		name string
		a    Adjustment
		want bool
	}{
		{"all zero except saturation 1", Adjustment{Saturation: 1.0}, true},
		{"hue set", Adjustment{Hue: 30, Saturation: 1.0}, false},
		{"lightness set", Adjustment{Lightness: -0.1, Saturation: 1.0}, false},
		{"saturation set", Adjustment{Saturation: 1.2}, false},
		{"saturation zero (grayscale)", Adjustment{Saturation: 0}, false},
	}
	for _, c := range cases {
		if got := c.a.IsNoOp(); got != c.want {
			t.Errorf("%s: IsNoOp() = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestSettingsRoundTrip(t *testing.T) {
	original := &Settings{
		Schema:  settingsSchemaURL,
		Version: 1,
		Aliases: map[string]Alias{
			"warm-adventure": {Base: "Adventure", Hue: 30, Lightness: -0.1, Saturation: 1.2},
		},
		SchemeDefaults: map[string]Adjustment{
			"Solarized Dark": {Saturation: 1.2},
		},
	}
	data, err := json.MarshalIndent(original, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got Settings
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Version != 1 {
		t.Errorf("version = %d, want 1", got.Version)
	}
	a := got.Aliases["warm-adventure"]
	if a.Base != "Adventure" || a.Hue != 30 || a.Lightness != -0.1 || a.Saturation != 1.2 {
		t.Errorf("alias round-trip: %+v", a)
	}
	d := got.SchemeDefaults["Solarized Dark"]
	if d.Saturation != 1.2 {
		t.Errorf("scheme default round-trip: %+v", d)
	}
}

func TestResolveBaselineAlias(t *testing.T) {
	s := &Settings{
		Aliases: map[string]Alias{
			"warm-adventure": {Base: "Adventure", Hue: 30, Lightness: -0.1, Saturation: 1.2},
		},
	}
	base, baseline, isAlias := s.resolveBaselineFor("warm-adventure")
	if base != "Adventure" {
		t.Errorf("base = %q, want Adventure", base)
	}
	if !isAlias {
		t.Errorf("isAlias = false, want true")
	}
	if baseline.Hue != 30 || baseline.Lightness != -0.1 || baseline.Saturation != 1.2 {
		t.Errorf("baseline = %+v", baseline)
	}
}

func TestResolveBaselineSchemeDefaults(t *testing.T) {
	s := &Settings{
		SchemeDefaults: map[string]Adjustment{
			"Adventure": {Lightness: -0.1, Saturation: 1.0},
		},
	}
	base, baseline, isAlias := s.resolveBaselineFor("Adventure")
	if base != "Adventure" {
		t.Errorf("base = %q, want Adventure", base)
	}
	if isAlias {
		t.Errorf("isAlias = true, want false (scheme defaults are not aliases)")
	}
	if baseline.Lightness != -0.1 {
		t.Errorf("baseline.Lightness = %v, want -0.1", baseline.Lightness)
	}
}

func TestResolveBaselinePassthrough(t *testing.T) {
	s := &Settings{}
	base, baseline, isAlias := s.resolveBaselineFor("Adventure")
	if base != "Adventure" || isAlias {
		t.Errorf("got base=%q isAlias=%v, want Adventure false", base, isAlias)
	}
	if !baseline.IsNoOp() {
		t.Errorf("baseline should be no-op, got %+v", baseline)
	}
}

func TestSettingsLoadSaveFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s, err := loadSettings()
	if err != nil {
		t.Fatalf("loadSettings (empty home): %v", err)
	}
	if len(s.Aliases) != 0 {
		t.Errorf("expected no aliases in fresh load")
	}
	s.Aliases = map[string]Alias{
		"a1": {Base: "Adventure", Hue: 10, Saturation: 1.0},
	}
	if err := s.save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := loadSettings()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.Aliases["a1"].Hue != 10 {
		t.Errorf("round-trip hue lost: %+v", got.Aliases["a1"])
	}
	if got.Schema != settingsSchemaURL {
		t.Errorf("schema URL not written, got %q", got.Schema)
	}
}
