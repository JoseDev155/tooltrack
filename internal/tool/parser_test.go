package tool

import "testing"

func TestParseVersion(t *testing.T) {
	cases := []struct {
		in   string
		want Version
	}{
		{"1.2.3", Version{1, 2, 3}},
		{"v1.2.3", Version{1, 2, 3}},
		{"release-1.2.3", Version{1, 2, 3}},
		{"version/1.2.3", Version{1, 2, 3}},
		{"bun-v1.1.30", Version{1, 1, 30}},
		{"1.2", Version{1, 2, 0}},
		{"7", Version{7, 0, 0}},
		{"2.60.1 (2024-10-18)", Version{2, 60, 1}},
		{"1.2.3-rc.1", Version{1, 2, 3}},
		{"24.9.2", Version{24, 9, 2}},
		{"  v0.5.10\n", Version{0, 5, 10}},
	}
	for _, c := range cases {
		got, err := ParseVersion(c.in)
		if err != nil || got != c.want {
			t.Errorf("ParseVersion(%q) = %v, %v; want %v", c.in, got, err, c.want)
		}
	}
}

func TestParseVersionErrors(t *testing.T) {
	for _, in := range []string{"", "latest", "v", "release-candidate"} {
		if _, err := ParseVersion(in); err == nil {
			t.Errorf("ParseVersion(%q): expected an error", in)
		}
	}
}

func TestVersionString(t *testing.T) {
	if got := (Version{1, 2, 0}).String(); got != "1.2.0" {
		t.Errorf("String() = %q", got)
	}
}

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b Version
		want int
	}{
		{Version{1, 2, 3}, Version{1, 2, 3}, 0},
		{Version{1, 2, 3}, Version{1, 2, 4}, -1},
		{Version{1, 2, 4}, Version{1, 2, 3}, 1},
		{Version{1, 3, 0}, Version{1, 2, 9}, 1},
		{Version{2, 0, 0}, Version{1, 99, 99}, 1},
		{Version{0, 9, 0}, Version{1, 0, 0}, -1},
		{Version{1, 10, 0}, Version{1, 9, 0}, 1},
	}
	for _, c := range cases {
		if got := c.a.Compare(c.b); got != c.want {
			t.Errorf("%v.Compare(%v) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

// The original spec used independent ORs between components, which reports
// an update whenever any single component is higher. These cases fail under
// that formula and pin the lexicographic behaviour.
func TestHasUpdateIsLexicographic(t *testing.T) {
	cases := []struct {
		local, remote string
		want          bool
	}{
		{"1.0.0", "1.0.1", true},
		{"1.0.0", "1.1.0", true},
		{"1.0.0", "2.0.0", true},
		{"1.2.3", "1.2.3", false},
		{"2.0.0", "1.5.0", false},  // minor is higher but major is lower
		{"1.5.0", "1.4.9", false},  // patch is higher but minor is lower
		{"2.3.4", "3.0.0", true},   // minor/patch are lower but major is higher
		{"1.9.9", "1.10.0", true},  // numeric, not textual, comparison
		{"v1.2.3", "1.2.3", false}, // prefixes are ignored
		{"1.2.3", "bun-v1.2.4", true},
		{"1.2", "1.2.0", false},
	}
	for _, c := range cases {
		got, err := HasUpdate(c.local, c.remote)
		if err != nil || got != c.want {
			t.Errorf("HasUpdate(%q, %q) = %v, %v; want %v", c.local, c.remote, got, err, c.want)
		}
	}
}

func TestHasUpdateErrors(t *testing.T) {
	if _, err := HasUpdate("", "1.0.0"); err == nil {
		t.Error("empty local version should fail")
	}
	if _, err := HasUpdate("1.0.0", "nightly"); err == nil {
		t.Error("unparseable remote version should fail")
	}
}

func TestExtractVersion(t *testing.T) {
	got, err := ExtractVersion("uv 0.5.1 (abc 2024-11-01)", `uv\s+(\d+\.\d+\.\d+)`)
	if err != nil || got != "0.5.1" {
		t.Errorf("got %q, %v", got, err)
	}
}

func TestExtractVersionErrors(t *testing.T) {
	if _, err := ExtractVersion("hello\nworld", `uv\s+(\d+)`); err == nil {
		t.Error("expected an error when nothing matches")
	}
	if _, err := ExtractVersion("x", `(`); err == nil {
		t.Error("expected an error for an invalid regex")
	}
	if _, err := ExtractVersion("abc", `abc`); err == nil {
		t.Error("expected an error when the regex has no capture group")
	}
}
