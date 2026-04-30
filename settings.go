package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	settingsFileName  = ".it2colors_settings.json"
	settingsSchemaURL = "https://raw.githubusercontent.com/jacksongilman/it2colors/main/schema/it2colors-settings.schema.json"
	settingsVersion   = 1
)

// Adjustment is a tuple of HSV-style transforms applied in CIE HCL space.
//
// Saturation's no-op value is 1.0, not the Go zero. UnmarshalJSON defaults
// missing saturation to 1.0 so hand-edited JSON like `{"hue": 30}` reads as
// "rotate hue, leave saturation alone." An explicit `"saturation": 0` is
// preserved (full grayscale).
type Adjustment struct {
	Hue        float64 `json:"hue"`
	Lightness  float64 `json:"lightness"`
	Saturation float64 `json:"saturation"`
}

func (a *Adjustment) UnmarshalJSON(data []byte) error {
	type raw Adjustment
	tmp := raw{Saturation: 1.0}
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}
	*a = Adjustment(tmp)
	return nil
}

func newAdjustment() Adjustment {
	return Adjustment{Saturation: 1.0}
}

func (a Adjustment) IsNoOp() bool {
	return a.Hue == 0 && a.Lightness == 0 && a.Saturation == 1.0
}

type Alias struct {
	Base       string  `json:"base"`
	Hue        float64 `json:"hue"`
	Lightness  float64 `json:"lightness"`
	Saturation float64 `json:"saturation"`
}

func (a *Alias) UnmarshalJSON(data []byte) error {
	type raw Alias
	tmp := raw{Saturation: 1.0}
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}
	*a = Alias(tmp)
	return nil
}

func (a Alias) Adjustment() Adjustment {
	return Adjustment{Hue: a.Hue, Lightness: a.Lightness, Saturation: a.Saturation}
}

type Settings struct {
	Schema         string                `json:"$schema,omitempty"`
	Version        int                   `json:"version"`
	Aliases        map[string]Alias      `json:"aliases,omitempty"`
	SchemeDefaults map[string]Adjustment `json:"scheme_defaults,omitempty"`
}

func settingsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, settingsFileName), nil
}

// loadSettings reads ~/.it2colors_settings.json. If the file is absent it
// returns a zero-value Settings ready to be populated and saved.
func loadSettings() (*Settings, error) {
	path, err := settingsPath()
	if err != nil {
		return nil, err
	}
	s := &Settings{Version: settingsVersion}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return s, nil
	}
	if err := json.Unmarshal(data, s); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if s.Version != 0 && s.Version != settingsVersion {
		return nil, fmt.Errorf("%s: unsupported version %d (expected %d)", path, s.Version, settingsVersion)
	}
	if s.Version == 0 {
		s.Version = settingsVersion
	}
	return s, nil
}

// save writes the settings atomically (temp file + rename) with mode 0644.
func (s *Settings) save() error {
	path, err := settingsPath()
	if err != nil {
		return err
	}
	if s.Schema == "" {
		s.Schema = settingsSchemaURL
	}
	if s.Version == 0 {
		s.Version = settingsVersion
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".it2colors_settings.*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, 0644); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}

// resolveBaselineFor returns the (base scheme, baseline adjustment, isAlias)
// for the given invoked name. If `name` matches an alias, the alias's base
// and saved adjustment are returned. Otherwise, if `name` has scheme defaults,
// they're used as the baseline. Otherwise the baseline is a no-op adjustment.
func (s *Settings) resolveBaselineFor(name string) (base string, baseline Adjustment, isAlias bool) {
	if s != nil {
		if a, ok := s.Aliases[name]; ok {
			return a.Base, a.Adjustment(), true
		}
		if d, ok := s.SchemeDefaults[name]; ok {
			return name, d, false
		}
	}
	return name, newAdjustment(), false
}

// aliasNames returns alias keys in arbitrary order.
func (s *Settings) aliasNames() []string {
	if s == nil {
		return nil
	}
	names := make([]string, 0, len(s.Aliases))
	for n := range s.Aliases {
		names = append(names, n)
	}
	return names
}
