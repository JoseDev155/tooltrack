package views

import (
	"strings"
	"testing"
)

func TestBannerRowsHaveEqualWidth(t *testing.T) {
	rows := banner("TOOLTRACK")
	for i, r := range rows {
		if got, want := len([]rune(r)), len([]rune(rows[0])); got != want {
			t.Errorf("row %d has %d columns, want %d", i, got, want)
		}
	}
}

func TestGlyphsAreRectangular(t *testing.T) {
	for r, g := range glyphs {
		w := len([]rune(g[0]))
		for i, row := range g {
			if len([]rune(row)) != w {
				t.Errorf("glyph %q row %d has width %d, want %d", r, i, len([]rune(row)), w)
			}
		}
	}
}

func TestMainMenuShowsLogoAndItems(t *testing.T) {
	out := MainMenu([]string{"One", "Two"}, 1)
	if lines := strings.Count(Logo(), "\n") + 1; lines != 3 {
		t.Errorf("logo has %d lines, want 3", lines)
	}
	for _, want := range []string{"One", "▸", "Two"} {
		if !strings.Contains(out, want) {
			t.Errorf("menu missing %q:\n%s", want, out)
		}
	}
	if !strings.HasPrefix(out, Logo()) {
		t.Error("menu must start with the logo")
	}
}
