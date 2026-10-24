package assets

import (
	"encoding/json"
	"testing"
)

func TestDefaultToolsIsJSONArray(t *testing.T) {
	var v []map[string]any
	if err := json.Unmarshal(DefaultTools(), &v); err != nil {
		t.Fatal(err)
	}
	if len(v) == 0 {
		t.Error("catalog is empty")
	}
}

func TestLocale(t *testing.T) {
	for _, lang := range []string{"es", "en"} {
		data, err := Locale(lang)
		if err != nil || len(data) == 0 {
			t.Errorf("Locale(%q) = %d bytes, %v", lang, len(data), err)
		}
	}
	if _, err := Locale("xx"); err == nil {
		t.Error("expected an error for an unknown locale")
	}
}
