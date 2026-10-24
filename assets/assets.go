// Package assets exposes the files embedded in the binary. It lives at the
// module root because go:embed cannot reference paths containing "..".
package assets

import (
	"embed"
	"fmt"
)

//go:embed defaults.json locales/*.json
var fs embed.FS

// DefaultTools returns the embedded catalog of supported tools.
func DefaultTools() []byte {
	data, err := fs.ReadFile("defaults.json")
	if err != nil {
		// The file is embedded at compile time, so this cannot happen.
		panic(err)
	}
	return data
}

// Locale returns the embedded translation file for lang ("es", "en").
func Locale(lang string) ([]byte, error) {
	data, err := fs.ReadFile("locales/" + lang + ".json")
	if err != nil {
		return nil, fmt.Errorf("unknown locale %q: %w", lang, err)
	}
	return data, nil
}
