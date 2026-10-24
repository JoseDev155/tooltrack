// Package views renders the screens of the TUI as strings. The functions are
// pure: they take plain data and know nothing about Bubble Tea.
package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/JoseDev155/tooltrack/internal/app"
	"github.com/JoseDev155/tooltrack/internal/i18n"
	"github.com/JoseDev155/tooltrack/internal/tool"
	"github.com/JoseDev155/tooltrack/internal/ui/styles"
)

const cursorMark = "▸ "

func pad(s string, width int) string {
	if w := lipgloss.Width(s); w < width {
		return s + strings.Repeat(" ", width-w)
	}
	return s
}

func or(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func prefix(active bool) string {
	if active {
		return styles.Cursor.Render(cursorMark)
	}
	return "  "
}

// Dashboard renders the table of tools: name, local version, latest version
// and status.
func Dashboard(tr *i18n.Translator, tools []*tool.Tool) string {
	headers := []string{tr.T("table.tool"), tr.T("table.local"), tr.T("table.latest"), tr.T("table.status")}
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = lipgloss.Width(h)
	}

	type row struct {
		cells  [4]string
		status app.Status
		err    string
	}
	rows := make([]row, len(tools))
	for i, t := range tools {
		st := app.StatusOf(t)
		rows[i] = row{
			cells:  [4]string{t.Name, or(t.InstalledVersion, "-"), or(t.RemoteVersion, "-"), "[" + tr.T(st.StatusKey()) + "]"},
			status: st,
			err:    t.LastError,
		}
		for c, cell := range rows[i].cells {
			if w := lipgloss.Width(cell); w > widths[c] {
				widths[c] = w
			}
		}
	}

	var b strings.Builder
	for i, h := range headers {
		b.WriteString(styles.Header.Render(pad(h, widths[i])))
		b.WriteString("   ")
	}
	b.WriteString("\n")
	for _, r := range rows {
		for c := 0; c < 3; c++ {
			b.WriteString(pad(r.cells[c], widths[c]) + "   ")
		}
		b.WriteString(statusStyle(r.status).Render(r.cells[3]))
		b.WriteString("\n")
		if r.err != "" {
			b.WriteString(styles.Muted.Render("  ↳ "+r.err) + "\n")
		}
	}
	return b.String()
}

func statusStyle(s app.Status) lipgloss.Style {
	switch s {
	case app.StatusUpToDate:
		return styles.OK
	case app.StatusUpdateAvailable:
		return styles.Warn
	case app.StatusNotInstalled:
		return styles.Muted
	}
	return styles.Err
}

// Menu renders a list of choices with a cursor.
func Menu(title string, items []string, cursor int) string {
	var b strings.Builder
	b.WriteString(styles.Title.Render(title) + "\n\n")
	for i, it := range items {
		line := it
		if i == cursor {
			line = styles.Selected.Render(it)
		}
		b.WriteString(prefix(i == cursor) + line + "\n")
	}
	return b.String()
}

// ChecklistItem is one row of a multiple-choice screen.
type ChecklistItem struct {
	Label   string
	Checked bool
}

// Checklist renders checkboxes with a cursor.
func Checklist(title string, items []ChecklistItem, cursor int) string {
	var b strings.Builder
	b.WriteString(styles.Title.Render(title) + "\n\n")
	for i, it := range items {
		box := "[ ] "
		if it.Checked {
			box = "[x] "
		}
		line := box + it.Label
		if i == cursor {
			line = styles.Selected.Render(line)
		}
		b.WriteString(prefix(i == cursor) + line + "\n")
	}
	return b.String()
}

// RowState is the progress of one tool during an update.
type RowState int

const (
	RowPending RowState = iota
	RowRunning
	RowDone
	RowFailed
)

// ProgressRow is one tool in the updating screen.
type ProgressRow struct {
	Name  string
	State RowState
	Err   string
}

// Progress renders one line per tool; spinner is the current frame of the
// animation shown next to running tools.
func Progress(title string, rows []ProgressRow, spinner string) string {
	var b strings.Builder
	b.WriteString(styles.Title.Render(title) + "\n\n")
	for _, r := range rows {
		switch r.State {
		case RowRunning:
			b.WriteString(spinner + " " + r.Name + "\n")
		case RowDone:
			b.WriteString(styles.OK.Render("✓") + " " + r.Name + "\n")
		case RowFailed:
			b.WriteString(styles.Err.Render("✗") + " " + r.Name + "\n")
		default:
			b.WriteString(styles.Muted.Render("· "+r.Name) + "\n")
		}
	}
	return b.String()
}

// Summary renders the final result of an update run. When any update failed
// it shows the log location.
func Summary(tr *i18n.Translator, results []app.UpdateResult, cancelled bool, logPath string) string {
	var b strings.Builder
	b.WriteString(styles.Title.Render(tr.T("finished.title")) + "\n\n")
	failed := false
	for _, r := range results {
		switch {
		case r.Err == nil:
			b.WriteString(styles.OK.Render("✓ "+r.Name) + "  " + styles.Muted.Render(tr.T("finished.ok")) + "\n")
		default:
			failed = true
			b.WriteString(styles.Err.Render("✗ "+r.Name) + "  " + styles.Muted.Render(firstLine(r.Err.Error())) + "\n")
		}
	}
	if cancelled {
		b.WriteString("\n" + styles.Warn.Render(tr.T("finished.cancelled")) + "\n")
	}
	if failed {
		b.WriteString("\n" + styles.Err.Render(tr.T("error.title")) + "\n")
		b.WriteString(tr.T("error.view_logs", logPath) + "\n")
	}
	return b.String()
}

func firstLine(s string) string {
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		return s[:i]
	}
	return s
}

// SettingRow is one preference: its label and current value.
type SettingRow struct {
	Label, Value string
}

// Settings renders the preferences screen. note is a short message such as
// "Saved" and hint explains where the other options live.
func Settings(title string, rows []SettingRow, cursor int, note, hint string) string {
	width := 0
	for _, r := range rows {
		if w := lipgloss.Width(r.Label); w > width {
			width = w
		}
	}
	var b strings.Builder
	b.WriteString(styles.Title.Render(title) + "\n\n")
	for i, r := range rows {
		line := pad(r.Label, width) + "  " + r.Value
		if i == cursor {
			line = styles.Selected.Render(line)
		}
		b.WriteString(prefix(i == cursor) + line + "\n")
	}
	if note != "" {
		b.WriteString("\n" + styles.OK.Render(note) + "\n")
	}
	if hint != "" {
		b.WriteString("\n" + styles.Muted.Render(hint) + "\n")
	}
	return b.String()
}

// Loading renders the lookup screen.
func Loading(spinner, text string) string {
	return fmt.Sprintf("%s %s\n", spinner, text)
}
