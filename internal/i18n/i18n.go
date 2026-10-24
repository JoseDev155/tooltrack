// Package i18n looks up translated strings from the embedded locale files.
package i18n

import (
	"encoding/json"
	"fmt"

	"github.com/JoseDev155/tooltrack/assets"
)

// Fallback is the language used for keys missing from the selected one.
const Fallback = "en"

// Translator holds one language in memory.
type Translator struct {
	lang     string
	messages map[string]string
	fallback map[string]string
}

// New loads lang ("es" or "en"). Keys absent from lang are answered from the
// English file.
func New(lang string) (*Translator, error) {
	messages, err := load(lang)
	if err != nil {
		return nil, err
	}
	fallback := messages
	if lang != Fallback {
		if fallback, err = load(Fallback); err != nil {
			return nil, err
		}
	}
	return &Translator{lang: lang, messages: messages, fallback: fallback}, nil
}

func load(lang string) (map[string]string, error) {
	data, err := assets.Locale(lang)
	if err != nil {
		return nil, err
	}
	var messages map[string]string
	if err := json.Unmarshal(data, &messages); err != nil {
		return nil, fmt.Errorf("i18n: parse locale %q: %w", lang, err)
	}
	return messages, nil
}

// Lang returns the language this translator was created with.
func (t *Translator) Lang() string { return t.lang }

// T returns the text for key. When args are given the text is used as a
// fmt format string. An unknown key is returned as is, so a missing
// translation is visible instead of blank.
func (t *Translator) T(key string, args ...any) string {
	text, ok := t.messages[key]
	if !ok {
		if text, ok = t.fallback[key]; !ok {
			return key
		}
	}
	if len(args) == 0 {
		return text
	}
	return fmt.Sprintf(text, args...)
}
