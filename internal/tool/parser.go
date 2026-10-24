package tool

import (
	"fmt"
	"regexp"
	"strconv"
)

// Version is a Major.Minor.Patch triple. Missing components are zero, so
// "1.2" equals "1.2.0".
type Version struct {
	Major, Minor, Patch int
}

func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// Compare returns -1, 0 or +1 when v is lower than, equal to or higher than
// other. Major is compared first, then Minor, then Patch.
func (v Version) Compare(other Version) int {
	for _, p := range [][2]int{{v.Major, other.Major}, {v.Minor, other.Minor}, {v.Patch, other.Patch}} {
		switch {
		case p[0] < p[1]:
			return -1
		case p[0] > p[1]:
			return 1
		}
	}
	return 0
}

// versionPattern finds the first number with up to three dot-separated parts.
// Searching, instead of trimming known prefixes, copes with tags such as
// "v1.2.3", "release-1.2.3", "version/1.2.3" and "bun-v1.1.30".
var versionPattern = regexp.MustCompile(`(\d+)(?:\.(\d+))?(?:\.(\d+))?`)

// ParseVersion extracts the first version number found in s.
func ParseVersion(s string) (Version, error) {
	m := versionPattern.FindStringSubmatch(s)
	if m == nil {
		return Version{}, fmt.Errorf("no version number in %q", s)
	}
	var parts [3]int
	for i := range parts {
		if m[i+1] == "" {
			continue
		}
		n, err := strconv.Atoi(m[i+1])
		if err != nil {
			return Version{}, fmt.Errorf("invalid version %q: %w", s, err)
		}
		parts[i] = n
	}
	return Version{parts[0], parts[1], parts[2]}, nil
}

// ExtractVersion applies expr to output and returns capture group 1.
func ExtractVersion(output, expr string) (string, error) {
	re, err := regexp.Compile(expr)
	if err != nil {
		return "", fmt.Errorf("invalid version regex: %w", err)
	}
	m := re.FindStringSubmatch(output)
	if m == nil || len(m) < 2 || m[1] == "" {
		return "", fmt.Errorf("version not found in output %q", firstLine(output))
	}
	return m[1], nil
}

// HasUpdate reports whether remote is strictly newer than local. Each side
// may carry a prefix such as "v" or "release-".
func HasUpdate(local, remote string) (bool, error) {
	l, err := ParseVersion(local)
	if err != nil {
		return false, fmt.Errorf("local version: %w", err)
	}
	r, err := ParseVersion(remote)
	if err != nil {
		return false, fmt.Errorf("remote version: %w", err)
	}
	return r.Compare(l) > 0, nil
}

func firstLine(s string) string {
	for i, r := range s {
		if r == '\n' || r == '\r' {
			return s[:i]
		}
	}
	return s
}
