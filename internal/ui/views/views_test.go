package views

import (
	"errors"
	"strings"
	"testing"

	"github.com/JoseDev155/tooltrack/internal/app"
	"github.com/JoseDev155/tooltrack/internal/i18n"
	"github.com/JoseDev155/tooltrack/internal/tool"
)

func tr(t *testing.T, lang string) *i18n.Translator {
	t.Helper()
	tr, err := i18n.New(lang)
	if err != nil {
		t.Fatal(err)
	}
	return tr
}

func contains(t *testing.T, view string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(view, w) {
			t.Errorf("view is missing %q:\n%s", w, view)
		}
	}
}

func TestDashboardColumnsAndStatuses(t *testing.T) {
	tools := []*tool.Tool{
		{Name: "Alpha", IsInstalled: true, InstalledVersion: "1.0.0", RemoteVersion: "2.0.0", HasUpdate: true},
		{Name: "A much longer tool name", IsInstalled: true, InstalledVersion: "3.1.4", RemoteVersion: "3.1.4"},
		{Name: "Gone"},
		{Name: "Broken", IsInstalled: true, InstalledVersion: "1.0.0", LastError: "boom"},
	}
	view := Dashboard(tr(t, "en"), tools)
	contains(t, view, "Tool", "Local version", "Latest version", "Status",
		"[New version available]", "[Up to date]", "[Not installed]", "[Not verified]", "↳ boom")

	// Columns line up: the local version starts at the same column on each row.
	col := -1
	for _, line := range strings.Split(view, "\n") {
		for _, v := range []string{"1.0.0", "3.1.4"} {
			if strings.HasPrefix(line, "Alpha") || strings.HasPrefix(line, "A much") {
				if i := strings.Index(line, v); i >= 0 {
					if col == -1 {
						col = i
					} else if col != i {
						t.Errorf("version column at %d and %d:\n%s", col, i, view)
					}
				}
			}
		}
	}
}

func TestDashboardSpanish(t *testing.T) {
	view := Dashboard(tr(t, "es"), []*tool.Tool{{Name: "Alpha"}})
	contains(t, view, "Herramienta", "Versión local", "[No instalada]")
}

func TestDashboardEmpty(t *testing.T) {
	if view := Dashboard(tr(t, "en"), nil); !strings.Contains(view, "Tool") {
		t.Errorf("headers expected even without tools:\n%s", view)
	}
}

func TestMenuMarksCursor(t *testing.T) {
	view := Menu("Title", []string{"one", "two"}, 1)
	lines := strings.Split(view, "\n")
	var one, two string
	for _, l := range lines {
		if strings.Contains(l, "one") {
			one = l
		}
		if strings.Contains(l, "two") {
			two = l
		}
	}
	if strings.Contains(one, "▸") || !strings.Contains(two, "▸") {
		t.Errorf("cursor on the wrong row:\n%s", view)
	}
}

func TestChecklist(t *testing.T) {
	view := Checklist("Pick", []ChecklistItem{{Label: "a", Checked: true}, {Label: "b"}}, 0)
	contains(t, view, "[x] a", "[ ] b")
}

func TestProgress(t *testing.T) {
	view := Progress("Working", []ProgressRow{
		{Name: "pending"}, {Name: "running", State: RowRunning}, {Name: "done", State: RowDone}, {Name: "failed", State: RowFailed},
	}, "*")
	contains(t, view, "Working", "· pending", "* running", "✓ done", "✗ failed")
}

func TestSummarySuccessHasNoLogHint(t *testing.T) {
	view := Summary(tr(t, "en"), []app.UpdateResult{{Name: "Alpha"}}, false, "/logs/updater.log")
	contains(t, view, "Update summary", "Alpha", "Updated successfully")
	if strings.Contains(view, "/logs/updater.log") || strings.Contains(view, "An error occurred") {
		t.Errorf("no failures, so no log hint expected:\n%s", view)
	}
}

func TestSummaryFailureShowsLogPathAndFirstErrorLine(t *testing.T) {
	results := []app.UpdateResult{
		{Name: "Alpha"},
		{Name: "Beta", Err: errors.New("exit status 2\nsecond line")},
	}
	view := Summary(tr(t, "es"), results, false, "/logs/updater.log")
	contains(t, view, "Ha ocurrido un error durante la actualización", "/logs/updater.log", "exit status 2")
	if strings.Contains(view, "second line") {
		t.Errorf("only the first error line belongs in the summary:\n%s", view)
	}
}

func TestSummaryCancelled(t *testing.T) {
	contains(t, Summary(tr(t, "en"), nil, true, "x"), "Cancelled")
}

func TestSettings(t *testing.T) {
	view := Settings("Preferences", []SettingRow{{"Language", "English"}, {"Default action", "Ask"}}, 1, "Saved", "edit /x/config.json")
	contains(t, view, "Preferences", "Language", "English", "Default action", "Saved", "/x/config.json")
}

func TestLoading(t *testing.T) {
	contains(t, Loading("*", "wait"), "* wait")
}
