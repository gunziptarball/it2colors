package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/lucasb-eyer/go-colorful"
	"howett.net/plist"
)

type Color struct {
	Red   float64 `plist:"Red Component"`
	Green float64 `plist:"Green Component"`
	Blue  float64 `plist:"Blue Component"`
}

type Profile map[string]Color

func main() {
	random := flag.Bool("r", false, "pick a random scheme (default when no name is given)")
	eval := flag.Bool("eval", false, "print 'export IT2COLORS_SCHEME=<name>' to stdout; OSC escape still goes to /dev/tty")
	list := flag.Bool("list", false, "list available scheme names and exit")
	dir := flag.String("schemes-dir", "", "override the schemes directory")
	hue := flag.Float64("hue", 0, "rotate every color's hue by this many degrees (in CIE HCL space; preserves perceived lightness)")
	quiet := flag.Bool("q", false, "don't print the 'Applied scheme: ...' status line")
	flag.Usage = usage
	flag.Parse()

	schemesDir, err := resolveSchemesDir(*dir)
	if err != nil {
		fail(err)
	}

	if *list {
		names, err := listSchemes(schemesDir)
		if err != nil {
			fail(err)
		}
		for _, n := range names {
			fmt.Println(strings.TrimSuffix(n, ".itermcolors"))
		}
		return
	}

	useRandom := *random || flag.NArg() == 0
	var profileFile string
	if useRandom {
		f, err := pickRandom(schemesDir)
		if err != nil {
			fail(err)
		}
		profileFile = f
	} else {
		arg := strings.TrimSuffix(flag.Arg(0), ".itermcolors")
		profileFile = arg + ".itermcolors"
	}

	profilePath := filepath.Join(schemesDir, "schemes", profileFile)
	profile, err := loadProfile(profilePath)
	if err != nil {
		fail(fmt.Errorf("loading %s: %w", profilePath, err))
	}
	profile = rotateHue(profile, *hue)
	name := strings.TrimSuffix(profileFile, ".itermcolors")

	tty, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		fail(fmt.Errorf("opening /dev/tty: %w", err))
	}
	defer tty.Close()
	if err := applyProfile(tty, profile); err != nil {
		fail(err)
	}

	if err := appendHistory(name); err != nil {
		fmt.Fprintln(os.Stderr, "it2colors: warning: history append failed:", err)
	}

	if !*quiet {
		msg := fmt.Sprintf("Applied scheme: %s", name)
		if *hue != 0 {
			msg += fmt.Sprintf(" (hue %+g°)", *hue)
		}
		fmt.Fprintln(os.Stderr, msg)
	}

	if *eval {
		fmt.Printf("export IT2COLORS_SCHEME=%s\n", name)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `Usage: it2colors [flags] [<scheme name>]

Apply an iTerm2 color scheme from the mbadolato/iterm2-color-schemes archive
(https://github.com/mbadolato/iterm2-color-schemes). With no name, picks one
at random.

Flags:
`)
	flag.PrintDefaults()
	fmt.Fprint(os.Stderr, `
Schemes directory resolution order:
  1. -schemes-dir flag
  2. $ITERM2_COLOR_SCHEMES_PATH
  3. newest ~/Downloads/mbadolato-iTerm2-Color-Schemes-*

Suggested .bashrc / .zshrc:
  if [ -t 1 ] && command -v it2colors >/dev/null; then
    eval "$(it2colors -r -eval)"
  fi
`)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "it2colors:", err)
	os.Exit(1)
}

func resolveSchemesDir(override string) (string, error) {
	for _, c := range []string{override, os.Getenv("ITERM2_COLOR_SCHEMES_PATH")} {
		if c != "" && hasSchemes(c) {
			return c, nil
		}
	}

	home, err := os.UserHomeDir()
	if err == nil {
		matches, _ := filepath.Glob(filepath.Join(home, "Downloads", "mbadolato-iTerm2-Color-Schemes-*"))
		sort.Slice(matches, func(i, j int) bool {
			ai, _ := os.Stat(matches[i])
			aj, _ := os.Stat(matches[j])
			return ai.ModTime().After(aj.ModTime())
		})
		for _, m := range matches {
			if hasSchemes(m) {
				return m, nil
			}
		}
	}

	return "", fmt.Errorf("no schemes directory found (set $ITERM2_COLOR_SCHEMES_PATH or unpack the archive into ~/Downloads)")
}

func hasSchemes(p string) bool {
	s, err := os.Stat(filepath.Join(p, "schemes"))
	return err == nil && s.IsDir()
}

func listSchemes(schemesDir string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(schemesDir, "schemes"))
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".itermcolors") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

func pickRandom(schemesDir string) (string, error) {
	names, err := listSchemes(schemesDir)
	if err != nil {
		return "", err
	}
	if len(names) == 0 {
		return "", fmt.Errorf("no .itermcolors files in %s/schemes", schemesDir)
	}
	return names[rand.IntN(len(names))], nil
}

func loadProfile(path string) (Profile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	p := Profile{}
	if _, err := plist.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	return p, nil
}

func applyProfile(w io.Writer, p Profile) error {
	keys := make([]string, 0, len(p))
	for k := range p {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var buf bytes.Buffer
	for _, k := range keys {
		slot, ok := slotFor(k)
		if !ok {
			continue
		}
		c := p[k]
		fmt.Fprintf(&buf, "\x1b]P%c%02x%02x%02x\x1b\\",
			slot, clampByte(c.Red), clampByte(c.Green), clampByte(c.Blue))
	}
	_, err := w.Write(buf.Bytes())
	return err
}

func slotFor(key string) (byte, bool) {
	switch key {
	case "Foreground Color":
		return 'g', true
	case "Background Color":
		return 'h', true
	case "Bold Color":
		return 'i', true
	case "Selection Color":
		return 'j', true
	case "Selected Text Color":
		return 'k', true
	case "Cursor Color":
		return 'l', true
	case "Cursor Text Color":
		return 'm', true
	}
	var n int
	if _, err := fmt.Sscanf(key, "Ansi %d Color", &n); err == nil && n >= 0 && n <= 15 {
		return "0123456789abcdef"[n], true
	}
	return 0, false
}

// HCL (not HSL) so lightness is preserved and contrast against the
// background is unchanged. Achromatic colors are passed through:
// their hue is undefined and rotating it would tint pure black/white/gray.
func rotateHue(p Profile, deg float64) Profile {
	if deg == 0 {
		return p
	}
	out := make(Profile, len(p))
	const achromatic = 1e-4
	for k, c := range p {
		col := colorful.Color{R: c.Red, G: c.Green, B: c.Blue}.Clamped()
		h, ch, l := col.Hcl()
		if ch < achromatic || math.IsNaN(h) {
			out[k] = c
			continue
		}
		h = math.Mod(h+deg, 360)
		if h < 0 {
			h += 360
		}
		r := colorful.Hcl(h, ch, l).Clamped()
		out[k] = Color{Red: r.R, Green: r.G, Blue: r.B}
	}
	return out
}

func clampByte(v float64) int {
	n := int(v * 256)
	if n > 255 {
		return 255
	}
	if n < 0 {
		return 0
	}
	return n
}

func appendHistory(name string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(home, ".it2colors_history"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "%s\t%s\n", time.Now().Format(time.RFC3339), name)
	return err
}
