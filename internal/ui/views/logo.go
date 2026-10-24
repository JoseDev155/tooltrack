package views

import (
	"strings"

	"github.com/JoseDev155/tooltrack/internal/ui/styles"
)

// glyphs is a three-row block font for the letters of the logo.
var glyphs = map[rune][3]string{
	't': {"▀█▀", " █ ", " ▀▀"},
	'o': {"█▀▀█", "█  █", "▀▀▀▀"},
	'l': {"█   ", "█   ", "▀▀▀▀"},
	'r': {"█▀▀▀", "█   ", "▀   "},
	'a': {"█▀▀█", "█▀▀█", "▀  ▀"},
	'c': {"█▀▀▀", "█   ", "▀▀▀▀"},
	'k': {"█ ▄▀", "██▄ ", "▀  ▀"},
}

// banner draws word in the block font, one string per row.
func banner(word string) [3]string {
	var rows [3]string
	for i, r := range word {
		g := glyphs[r]
		for row := range rows {
			if i > 0 {
				rows[row] += " "
			}
			rows[row] += g[row]
		}
	}
	return rows
}

// Logo renders the tool name as block letters, "tool" muted and "track" in
// the accent colour.
func Logo() string {
	tool, track := banner("tool"), banner("track")
	lines := make([]string, len(tool))
	for i := range tool {
		lines[i] = styles.Muted.Render(tool[i]) + " " + styles.Title.Render(track[i])
	}
	return strings.Join(lines, "\n")
}

// MainMenu renders the start screen: the logo above the list of choices.
func MainMenu(items []string, cursor int) string {
	var b strings.Builder
	b.WriteString(Logo() + "\n\n")
	for i, it := range items {
		line := it
		if i == cursor {
			line = styles.Selected.Render(it)
		}
		b.WriteString(prefix(i == cursor) + line + "\n")
	}
	return b.String()
}
