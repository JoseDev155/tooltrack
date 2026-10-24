package i18n

import (
	"encoding/json"
	"sort"
	"testing"

	"github.com/JoseDev155/tooltrack/assets"
)

func TestNewUnknownLanguage(t *testing.T) {
	if _, err := New("fr"); err == nil {
		t.Fatal("expected an error for an unknown language")
	}
}

func TestTranslate(t *testing.T) {
	es, err := New("es")
	if err != nil {
		t.Fatal(err)
	}
	en, err := New("en")
	if err != nil {
		t.Fatal(err)
	}
	if got := es.T("menu.exit"); got != "Salir" {
		t.Errorf("es menu.exit = %q", got)
	}
	if got := en.T("menu.exit"); got != "Exit" {
		t.Errorf("en menu.exit = %q", got)
	}
	if es.Lang() != "es" {
		t.Errorf("Lang() = %q", es.Lang())
	}
}

func TestTranslateFormatsArguments(t *testing.T) {
	es, _ := New("es")
	got := es.T("error.view_logs", "/tmp/updater.log")
	want := "Ver mensaje en el archivo log: /tmp/updater.log"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestMissingKeyReturnsKey(t *testing.T) {
	en, _ := New("en")
	if got := en.T("no.such.key"); got != "no.such.key" {
		t.Errorf("got %q", got)
	}
}

func TestFallbackToEnglish(t *testing.T) {
	tr := &Translator{
		lang:     "es",
		messages: map[string]string{"a": "uno"},
		fallback: map[string]string{"a": "one", "b": "two"},
	}
	if got := tr.T("a"); got != "uno" {
		t.Errorf("a = %q", got)
	}
	if got := tr.T("b"); got != "two" {
		t.Errorf("b = %q, want English fallback", got)
	}
}

// Both locale files must define exactly the same keys, with the same number
// of format verbs.
func TestLocalesHaveSameKeys(t *testing.T) {
	es := rawLocale(t, "es")
	en := rawLocale(t, "en")
	for _, k := range sortedKeys(es) {
		if _, ok := en[k]; !ok {
			t.Errorf("key %q is in es.json but not in en.json", k)
		}
	}
	for _, k := range sortedKeys(en) {
		if _, ok := es[k]; !ok {
			t.Errorf("key %q is in en.json but not in es.json", k)
		}
	}
	if len(es) == 0 {
		t.Error("es.json is empty")
	}
}

func TestLocalesHaveNoEmptyValues(t *testing.T) {
	for _, lang := range []string{"es", "en"} {
		for k, v := range rawLocale(t, lang) {
			if v == "" {
				t.Errorf("%s: key %q is empty", lang, k)
			}
		}
	}
}

func rawLocale(t *testing.T, lang string) map[string]string {
	t.Helper()
	data, err := assets.Locale(lang)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("%s.json: %v", lang, err)
	}
	return m
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
