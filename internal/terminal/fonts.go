package terminal

import (
	"runtime"
	"sort"
	"strings"

	"github.com/egoist/mygo/ui"
)

// terminalFontFamily keeps the chosen text face first and gives private-use
// prompt symbols an installed Nerd Font before Core Text's system fallback.
// FontFamilies is thread-safe and caches system discovery; this runs only when
// creating a terminal or changing its font, never while painting a frame.
func terminalFontFamily(preferred string) string {
	return fontFamilyWithSymbols(preferred, ui.FontFamilies())
}

func fontFamilyWithSymbols(preferred string, installed []ui.FontFamily) string {
	defaultMono := "monospace"
	if runtime.GOOS == "darwin" {
		// NSFont's system-monospaced descriptor does not honor the custom
		// symbol cascade on current macOS. A real named face does. Never use
		// an unavailable name, which would silently promote the symbol face.
		for _, name := range []string{"SF Mono", "Menlo"} {
			for _, family := range installed {
				if strings.EqualFold(family.Name, name) && family.Monospaced {
					defaultMono = family.Name
					break
				}
			}
			if defaultMono != "monospace" {
				break
			}
		}
	}
	families := make([]string, 0, 3)
	seen := map[string]bool{}
	appendFamily := func(name string) {
		name = strings.TrimSpace(name)
		if defaultMono != "monospace" && strings.EqualFold(name, "monospace") {
			name = defaultMono
		}
		key := strings.ToLower(name)
		if name != "" && !seen[key] {
			families = append(families, name)
			seen[key] = true
		}
	}
	for _, name := range strings.Split(preferred, ",") {
		appendFamily(name)
	}
	appendFamily(defaultMono)
	var candidates []string
	for _, family := range installed {
		name := strings.ToLower(family.Name)
		if strings.Contains(name, "nerd font") && (family.Monospaced || strings.HasSuffix(name, " mono") || name == "symbols nerd font") {
			candidates = append(candidates, family.Name)
		}
	}
	priority := func(name string) int {
		switch strings.ToLower(name) {
		case "symbols nerd font mono":
			return 0
		case "symbols nerd font":
			return 1
		default:
			return 2
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if a, b := priority(candidates[i]), priority(candidates[j]); a != b {
			return a < b
		}
		return strings.ToLower(candidates[i]) < strings.ToLower(candidates[j])
	})
	if len(candidates) > 0 {
		appendFamily(candidates[0])
	}
	return strings.Join(families, ", ")
}
