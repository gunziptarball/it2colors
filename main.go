package main

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"github.com/lucasb-eyer/go-colorful"
	flag "github.com/spf13/pflag"
	"howett.net/plist"
)

type Color struct {
	Red   float64 `plist:"Red Component"`
	Green float64 `plist:"Green Component"`
	Blue  float64 `plist:"Blue Component"`
}

type Profile map[string]Color

func main() {
	random := flag.BoolP("random", "r", false, "pick a random scheme (default when no name is given)")
	current := flag.BoolP("current", "c", false, "use the current scheme from $IT2COLORS_SCHEME (set by --eval)")
	eval := flag.Bool("eval", false, "print 'export IT2COLORS_SCHEME=<name>' to stdout; OSC escape still goes to /dev/tty")
	list := flag.BoolP("list", "l", false, "list available scheme names and exit")
	dir := flag.String("schemes-dir", "", "override the schemes directory")
	hue := flag.Float64("hue", 0, "rotate every color's hue by this many degrees (CIE HCL; preserves perceived lightness)")
	lightness := flag.Float64("lightness", 0, "shift every color's perceived lightness (CIE HCL L, 0–1 range; negative darkens, e.g. -0.2)")
	saturation := flag.Float64("saturation", 1, "scale every color's chroma/saturation (CIE HCL; 1.0 = no change, <1 desaturates, >1 boosts)")
	quiet := flag.BoolP("quiet", "q", false, "don't print the 'Applied scheme: ...' status line")
	preview := flag.BoolP("preview", "p", false, "after applying, print a foreground × background SGR test table to verify the scheme")
	favorite := flag.BoolP("favorite", "f", false, "add $IT2COLORS_SCHEME to favorites (used by default random pool when non-empty)")
	unfavorite := flag.Bool("unfavorite", false, "remove $IT2COLORS_SCHEME from favorites")
	yuck := flag.Bool("yuck", false, "blacklist $IT2COLORS_SCHEME from future picks; combine with -r to immediately move to a new scheme")
	all := flag.Bool("all", false, "pick from all schemes, ignoring the favorites list (yuck list still applies)")
	showVersion := flag.BoolP("version", "v", false, "print version and exit")
	flag.Usage = usage
	flag.Parse()

	if *showVersion {
		fmt.Println(buildVersion())
		return
	}

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

	// Management flags: operate on $IT2COLORS_SCHEME and exit (--yuck continues if -r is also set).
	if *favorite || *unfavorite || *yuck {
		scheme, err := currentSchemeName()
		if err != nil {
			fail(err)
		}
		home, err := os.UserHomeDir()
		if err != nil {
			fail(err)
		}
		favPath := filepath.Join(home, ".it2colors_favorites")
		yuckPath := filepath.Join(home, ".it2colors_yuck")

		if *favorite {
			if err := addToNameFile(favPath, scheme); err != nil {
				fmt.Fprintln(os.Stderr, "it2colors: warning: could not update favorites:", err)
			}
			fmt.Fprintf(os.Stderr, "Favorited: %s\n", scheme)
			return
		}
		if *unfavorite {
			if err := removeFromNameFile(favPath, scheme); err != nil {
				fmt.Fprintln(os.Stderr, "it2colors: warning: could not update favorites:", err)
			}
			fmt.Fprintf(os.Stderr, "Unfavorited: %s\n", scheme)
			return
		}
		if *yuck {
			if err := removeFromNameFile(favPath, scheme); err != nil {
				fmt.Fprintln(os.Stderr, "it2colors: warning: could not update favorites:", err)
			}
			if err := addToNameFile(yuckPath, scheme); err != nil {
				fmt.Fprintln(os.Stderr, "it2colors: warning: could not update yuck list:", err)
			}
			fmt.Fprintf(os.Stderr, "Yucked: %s\n", scheme)
			if !*random && flag.NArg() == 0 {
				return
			}
			// fall through to pick a new scheme
		}
	}

	if *current && (*random || flag.NArg() > 0) {
		fail(fmt.Errorf("--current cannot be combined with --random or a scheme name"))
	}

	var profileFile string
	switch {
	case *current:
		scheme, err := currentSchemeName()
		if err != nil {
			fail(err)
		}
		profileFile = scheme + ".itermcolors"
	case *random || flag.NArg() == 0:
		f, err := pickRandom(schemesDir, *all)
		if err != nil {
			fail(err)
		}
		profileFile = f
	default:
		arg := strings.TrimSuffix(flag.Arg(0), ".itermcolors")
		profileFile = arg + ".itermcolors"
	}

	profilePath := filepath.Join(schemesDir, "schemes", profileFile)
	profile, err := loadProfile(profilePath)
	if err != nil {
		fail(fmt.Errorf("loading %s: %w", profilePath, err))
	}
	profile = adjustColors(profile, *hue, *lightness, *saturation)
	name := strings.TrimSuffix(profileFile, ".itermcolors")

	tty, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		fail(fmt.Errorf("opening /dev/tty: %w", err))
	}
	defer tty.Close()
	if err := applyProfile(tty, profile); err != nil {
		fail(err)
	}
	if *preview {
		if err := printPreview(tty); err != nil {
			fail(err)
		}
	}

	if err := appendHistory(name); err != nil {
		fmt.Fprintln(os.Stderr, "it2colors: warning: history append failed:", err)
	}

	if !*quiet {
		msg := fmt.Sprintf("Applied scheme: %s", name)
		var adj []string
		if *hue != 0 {
			adj = append(adj, fmt.Sprintf("hue %+g°", *hue))
		}
		if *lightness != 0 {
			adj = append(adj, fmt.Sprintf("lightness %+g", *lightness))
		}
		if *saturation != 1 {
			adj = append(adj, fmt.Sprintf("saturation ×%.2g", *saturation))
		}
		if len(adj) > 0 {
			msg += " (" + strings.Join(adj, ", ") + ")"
		}
		fmt.Fprintln(os.Stderr, msg)
	}

	if *eval {
		fmt.Printf("export IT2COLORS_SCHEME='%s'\n", name)
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
  1. --schemes-dir flag
  2. $ITERM2_COLOR_SCHEMES_PATH
  3. newest ~/Downloads/mbadolato-iTerm2-Color-Schemes-*

Suggested .bashrc / .zshrc:
  if [ -t 1 ] && command -v it2colors >/dev/null; then
    eval "$(it2colors -r --eval)"
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

func pickRandom(schemesDir string, useAll bool) (string, error) {
	allNames, err := listSchemes(schemesDir) // filenames with .itermcolors extension
	if err != nil {
		return "", err
	}
	if len(allNames) == 0 {
		return "", fmt.Errorf("no .itermcolors files in %s/schemes", schemesDir)
	}

	home, _ := os.UserHomeDir()
	yuck := loadNameSet(filepath.Join(home, ".it2colors_yuck"))

	// If favorites are defined and --all wasn't requested, restrict to that pool.
	if !useAll {
		favs := loadNameSet(filepath.Join(home, ".it2colors_favorites"))
		if len(favs) > 0 {
			// Validate against available schemes so stale favorites don't cause errors.
			valid := make(map[string]bool, len(allNames))
			for _, n := range allNames {
				valid[strings.TrimSuffix(n, ".itermcolors")] = true
			}
			var pool []string
			for name := range favs {
				if valid[name] && !yuck[name] {
					pool = append(pool, name+".itermcolors")
				}
			}
			if len(pool) > 0 {
				sort.Strings(pool)
				return pool[rand.IntN(len(pool))], nil
			}
			// All favorites were yucked or invalid — fall through to full pool.
		}
	}

	// Full pool: exclude yuck, then apply recent-history exclusion.
	pool := make([]string, 0, len(allNames))
	for _, n := range allNames {
		if !yuck[strings.TrimSuffix(n, ".itermcolors")] {
			pool = append(pool, n)
		}
	}
	if len(pool) == 0 {
		pool = allNames // every scheme is yucked — give up and use all
	}
	if recent := recentHistory(20); len(recent) > 0 {
		skip := make(map[string]bool, len(recent))
		for _, n := range recent {
			skip[n+".itermcolors"] = true
		}
		var filtered []string
		for _, n := range pool {
			if !skip[n] {
				filtered = append(filtered, n)
			}
		}
		if len(filtered) > 0 {
			pool = filtered
		}
	}
	return pool[rand.IntN(len(pool))], nil
}

func buildVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" {
		return "(devel)"
	}
	return info.Main.Version
}

func currentSchemeName() (string, error) {
	name := os.Getenv("IT2COLORS_SCHEME")
	if name == "" {
		return "", fmt.Errorf("$IT2COLORS_SCHEME is not set; add `eval \"$(it2colors -r --eval)\"` to your shell startup")
	}
	return name, nil
}

// loadNameSet reads a newline-separated name file into a set.
// Returns nil if the file is absent or unreadable (treated as empty).
func loadNameSet(path string) map[string]bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	set := map[string]bool{}
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		if line != "" {
			set[line] = true
		}
	}
	return set
}

func writeNameFile(path string, set map[string]bool) error {
	names := make([]string, 0, len(set))
	for n := range set {
		names = append(names, n)
	}
	sort.Strings(names)
	var content string
	if len(names) > 0 {
		content = strings.Join(names, "\n") + "\n"
	}
	return os.WriteFile(path, []byte(content), 0644)
}

func addToNameFile(path, name string) error {
	set := loadNameSet(path)
	if set == nil {
		set = map[string]bool{}
	}
	if set[name] {
		return nil
	}
	set[name] = true
	return writeNameFile(path, set)
}

func removeFromNameFile(path, name string) error {
	set := loadNameSet(path)
	if set == nil || !set[name] {
		return nil
	}
	delete(set, name)
	return writeNameFile(path, set)
}

// recentHistory returns the scheme names from the last n lines of the history
// file, most-recent first. Returns nil if the file is absent or unreadable.
func recentHistory(n int) []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(home, ".it2colors_history"))
	if err != nil {
		return nil
	}
	return parseRecentHistory(data, n)
}

func parseRecentHistory(data []byte, n int) []string {
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	var recent []string
	for i := len(lines) - 1; i >= 0 && len(recent) < n; i-- {
		parts := strings.SplitN(lines[i], "\t", 2)
		if len(parts) == 2 && parts[1] != "" {
			recent = append(recent, parts[1])
		}
	}
	return recent
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

// adjustColors applies hue rotation, lightness shift, and saturation scaling
// in CIE HCL space. Using HCL (not HSL) preserves perceived contrast.
//
// Lightness shift applies to all colors including achromatic grays, so
// darkening a bright theme dims the background as intended. Hue rotation and
// saturation scaling are skipped for achromatic colors (chroma ≈ 0) because
// their hue is undefined and saturation is already zero.
func adjustColors(p Profile, hueDeg, lightnessDelta, saturationFactor float64) Profile {
	if hueDeg == 0 && lightnessDelta == 0 && saturationFactor == 1 {
		return p
	}
	out := make(Profile, len(p))
	const achromatic = 1e-4
	for k, c := range p {
		col := colorful.Color{R: c.Red, G: c.Green, B: c.Blue}.Clamped()
		h, ch, l := col.Hcl()

		chromatic := ch >= achromatic && !math.IsNaN(h)
		if chromatic {
			if hueDeg != 0 {
				h = math.Mod(h+hueDeg, 360)
				if h < 0 {
					h += 360
				}
			}
			ch = math.Max(0, ch*saturationFactor)
		}

		l = clamp01(l + lightnessDelta)

		if !chromatic && lightnessDelta == 0 {
			out[k] = c
			continue
		}
		if !chromatic {
			h = 0 // hue is irrelevant when chroma is 0
		}
		r := colorful.Hcl(h, ch, l).Clamped()
		out[k] = Color{Red: r.R, Green: r.G, Blue: r.B}
	}
	return out
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// Foreground × background SGR table, ported from the
// mbadolato/iterm2-color-schemes archive's tools/screenshotTable.sh
// (originally http://tldp.org/HOWTO/Bash-Prompt-HOWTO/x329.html).
// This is the same content that the per-scheme PNGs in screenshots/
// were captured from.
func printPreview(w io.Writer) error {
	const cell = "gYw"
	fgs := []string{
		"    m", "   1m",
		"  30m", "1;30m", "  31m", "1;31m", "  32m", "1;32m", "  33m", "1;33m",
		"  34m", "1;34m", "  35m", "1;35m", "  36m", "1;36m", "  37m", "1;37m",
	}
	bgs := []string{"40m", "41m", "42m", "43m", "44m", "45m", "46m", "47m"}

	var buf bytes.Buffer
	fmt.Fprint(&buf, "\n         def     40m     41m     42m     43m     44m     45m     46m     47m\n")
	for _, fgPad := range fgs {
		fg := strings.TrimSpace(fgPad)
		fmt.Fprintf(&buf, " %s \x1b[%s  %s  ", fgPad, fg, cell)
		for _, bg := range bgs {
			fmt.Fprintf(&buf, " \x1b[%s\x1b[%s  %s  \x1b[0m", fg, bg, cell)
		}
		buf.WriteByte('\n')
	}
	buf.WriteByte('\n')
	_, err := w.Write(buf.Bytes())
	return err
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
