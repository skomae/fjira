package issues

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/mk-5/fjira/internal/app"
	"github.com/mk-5/fjira/internal/jira"
	"github.com/mk-5/fjira/internal/ui"
	"github.com/stretchr/testify/assert"
)

func Test_detailLabelWidth(t *testing.T) {
	assert.Equal(t, len("Priority"), detailLabelWidth([]detailRow{
		{label: "Type"},
		{label: "Priority"},
		{label: "Created"},
	}))
	assert.Equal(t, 0, detailLabelWidth(nil))
}

func Test_buildDetailRows_content(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	issue := &jira.Issue{}
	issue.Fields.Priority.Name = "High"
	issue.Fields.Type.Name = "Bug"
	issue.Fields.Created = "2026-07-07T12:00:00.000+0000"
	issue.Fields.Updated = "2026-07-10T10:00:00.000+0000"
	rows := buildDetailRows(issue, now)
	// value = relative (primary), dimValue = absolute (rendered dimmer).
	assert.Equal(t, []detailRow{
		{label: ui.MessageDetailPriority, value: "High"},
		{label: ui.MessageDetailType, value: "Bug"},
		{label: ui.MessageDetailCreated, value: "3 days ago", dimValue: "7 Jul 2026 12:00 PM +0000"},
		{label: ui.MessageDetailUpdated, value: "2 hours ago", dimValue: "10 Jul 2026 10:00 AM +0000"},
	}, rows)
}

func Test_buildDetailRows_empty_timestamps(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	issue := &jira.Issue{} // no created/updated set
	rows := buildDetailRows(issue, now)
	// No parent set, so timestamps are at indices 2 and 3 (Priority, Type, then
	// Created, Updated). Empty timestamps yield empty value AND dimValue, so
	// Draw emits no stray "()".
	assert.Len(t, rows, 4)
	assert.Empty(t, rows[2].value)
	assert.Empty(t, rows[2].dimValue)
	assert.Empty(t, rows[3].value)
	assert.Empty(t, rows[3].dimValue)
}

func Test_parentDetailRow(t *testing.T) {
	t.Run("no parent -> omitted", func(t *testing.T) {
		_, ok := parentDetailRow(&jira.Issue{})
		assert.False(t, ok)
	})

	t.Run("epic parent -> Epic label, summary + key", func(t *testing.T) {
		issue := &jira.Issue{}
		issue.Fields.Parent.Key = "COINS-100"
		issue.Fields.Parent.Fields.Summary = "Auth Revamp"
		issue.Fields.Parent.Fields.Type.Name = "Epic"
		row, ok := parentDetailRow(issue)
		assert.True(t, ok)
		assert.Equal(t, detailRow{label: ui.MessageDetailEpic, value: "Auth Revamp", dimValue: "COINS-100"}, row)
	})

	t.Run("ticket parent -> Parent label, title + key", func(t *testing.T) {
		issue := &jira.Issue{}
		issue.Fields.Parent.Key = "COINS-50"
		issue.Fields.Parent.Fields.Summary = "Fix the login flow"
		issue.Fields.Parent.Fields.Type.Name = "Task"
		row, ok := parentDetailRow(issue)
		assert.True(t, ok)
		assert.Equal(t, detailRow{label: ui.MessageDetailParent, value: "Fix the login flow", dimValue: "COINS-50"}, row)
	})

	t.Run("epic type match is case-insensitive", func(t *testing.T) {
		issue := &jira.Issue{}
		issue.Fields.Parent.Key = "COINS-7"
		issue.Fields.Parent.Fields.Type.Name = "epic"
		row, _ := parentDetailRow(issue)
		assert.Equal(t, ui.MessageDetailEpic, row.label)
	})
}

func Test_buildDetailRows_prepends_parent(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	issue := &jira.Issue{}
	issue.Fields.Parent.Key = "COINS-100"
	issue.Fields.Parent.Fields.Summary = "Auth Revamp"
	issue.Fields.Parent.Fields.Type.Name = "Epic"
	rows := buildDetailRows(issue, now)
	// Parent leads the box, so it precedes Priority/Type/Created/Updated.
	assert.Len(t, rows, 5)
	assert.Equal(t, detailRow{label: ui.MessageDetailEpic, value: "Auth Revamp", dimValue: "COINS-100"}, rows[0])
	assert.Equal(t, ui.MessageDetailPriority, rows[1].label)
}

// renderVisibleRows draws the view and returns the visible screen buffer split
// into rows, spaces for empty cells.
func renderVisibleRows(view *issueView, screen tcell.SimulationScreen) []string {
	screen.Clear()
	view.Draw(screen)
	screen.Show()
	contents, cw, ch := screen.GetContents()
	rows := make([]string, ch)
	for y := 0; y < ch; y++ {
		var b bytes.Buffer
		for x := 0; x < cw; x++ {
			cell := contents[y*cw+x]
			if len(cell.Bytes) == 0 {
				b.WriteByte(' ')
			} else {
				b.Write(cell.Bytes)
			}
		}
		rows[y] = b.String()
	}
	return rows
}

// newDetailTestScreen returns a sim screen wired to the app at the given size.
// SetSize must run AFTER InitTestApp because InitTestApp re-runs screen.Init(),
// which resets the simulation screen back to the default 80x25.
func newDetailTestScreen(t *testing.T, w, h int) tcell.SimulationScreen {
	t.Helper()
	screen := tcell.NewSimulationScreen("utf-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	app.InitTestApp(screen)
	screen.SetSize(w, h)
	app.GetApp().ScreenX = w
	app.GetApp().ScreenY = h
	return screen
}

func detailTestIssue(comments int) *jira.Issue {
	issue := &jira.Issue{Key: "JWC-9", Id: "9"}
	issue.Fields.Summary = "A summary line"
	issue.Fields.Description = "Line1\nLine2\nLine3\nLine4\nLine5"
	issue.Fields.Type.Name = "Bug"
	issue.Fields.Priority.Name = "High"
	issue.Fields.Created = "2021-10-02T22:34:22.521+0200"
	issue.Fields.Updated = "2022-02-22T00:27:19.792+0100"
	issue.Fields.Status = jira.Status{Name: "Done"}
	issue.Fields.Labels = []string{"alpha", "beta"}
	issue.Fields.Parent.Key = "JWC-1"
	issue.Fields.Parent.Fields.Summary = "Parent epic title"
	issue.Fields.Parent.Fields.Type.Name = "Epic"
	for i := 1; i <= comments; i++ {
		issue.Fields.Comment.Comments = append(issue.Fields.Comment.Comments, jira.Comment{
			Body:    fmt.Sprintf("COMMENT-MARKER-%d body text", i),
			Created: "2022-06-09T22:53:42.057+0200",
			Author:  jira.User{DisplayName: "Alice"},
		})
	}
	return issue
}

// The Details box renders inside its border with the metadata rows.
func Test_issueView_draws_details_box(t *testing.T) {
	const w, h = 80, 60
	screen := newDetailTestScreen(t, w, h)
	defer screen.Fini()
	view := NewIssueView(detailTestIssue(1), nil, jira.NewJiraApiMock(nil)).(*issueView)
	view.Resize(w, h)

	joined := strings.Join(renderVisibleRows(view, screen), "\n")
	for _, want := range []string{ui.MessageDetails, "Priority", "High", "Bug"} {
		assert.Contains(t, joined, want, "Details box should render %q", want)
	}
}

// The parent/epic link renders as its own Details row: the summary as the
// primary value and the key in the dimmer style. The fixture's parent is an
// Epic, so the row is labelled "Epic".
func Test_issueView_draws_parent_row(t *testing.T) {
	const w, h = 100, 60
	screen := newDetailTestScreen(t, w, h)
	defer screen.Fini()
	view := NewIssueView(detailTestIssue(1), nil, jira.NewJiraApiMock(nil)).(*issueView)
	view.Resize(w, h)
	renderVisibleRows(view, screen)

	joined := strings.Join(renderVisibleRows(view, screen), "\n")
	assert.Contains(t, joined, ui.MessageDetailEpic, "parent row should be labelled Epic")
	assert.Contains(t, joined, "Parent epic title", "parent summary should render")
	assert.Contains(t, joined, "JWC-1", "parent key should render")

	// The parent key is the dim parenthetical; the summary uses the default fg.
	sumFg, sumOk := fgOnRowContaining(screen, "JWC-1", "Parent epic title")
	keyFg, keyOk := fgOnRowContaining(screen, "JWC-1", "JWC-1")
	assert.True(t, sumOk && keyOk, "parent row parts should be on screen")
	assert.Equal(t, app.Color("default.foreground"), sumFg, "parent summary uses the default foreground")
	assert.Equal(t, app.Color("details.foreground"), keyFg, "parent key uses the dim header color")
}

// fgOnRowContaining returns the foreground color of the first cell of `needle`,
// searching only the row that contains `anchor`. Scoping to a row disambiguates
// text that also appears elsewhere on screen (e.g. the relative time shown both
// in the top bar and in the Details row).
func fgOnRowContaining(screen tcell.SimulationScreen, anchor, needle string) (tcell.Color, bool) {
	contents, cw, ch := screen.GetContents()
	for y := 0; y < ch; y++ {
		var row bytes.Buffer
		offsets := make([]int, cw)
		for x := 0; x < cw; x++ {
			offsets[x] = row.Len()
			cell := contents[y*cw+x]
			if len(cell.Bytes) == 0 {
				row.WriteByte(' ')
			} else {
				row.Write(cell.Bytes)
			}
		}
		if !bytes.Contains(row.Bytes(), []byte(anchor)) {
			continue
		}
		idx := bytes.Index(row.Bytes(), []byte(needle))
		if idx < 0 {
			return tcell.ColorDefault, false
		}
		for x, off := range offsets {
			if off == idx {
				fg, _, _ := contents[y*cw+x].Style.Decompose()
				return fg, true
			}
		}
	}
	return tcell.ColorDefault, false
}

// The absolute date renders in a dimmer style (details.foreground, the dim gray
// used for box headers) than the relative time (default.foreground) — the whole
// point of this two-tone row. Both are scoped to the Details row (anchored by
// the absolute date, which is unique to it) so the top bar's "Updated: 4 years
// ago" can't be mismatched.
func Test_issueView_details_absolute_date_is_dimmer(t *testing.T) {
	const w, h = 100, 60
	screen := newDetailTestScreen(t, w, h)
	defer screen.Fini()
	view := NewIssueView(detailTestIssue(1), nil, jira.NewJiraApiMock(nil)).(*issueView)
	view.Resize(w, h)
	renderVisibleRows(view, screen) // paint the screen

	const absolute = "2 Oct 2021" // Created row's absolute date, unique to Details
	relFg, relOk := fgOnRowContaining(screen, absolute, "4 years ago")
	absFg, absOk := fgOnRowContaining(screen, absolute, absolute)
	assert.True(t, relOk, "relative time should be on the Details row")
	assert.True(t, absOk, "absolute date should be on the Details row")
	assert.Equal(t, app.Color("default.foreground"), relFg, "relative time uses the default foreground")
	assert.Equal(t, app.Color("details.foreground"), absFg, "absolute date uses the dim header color")
	assert.NotEqual(t, relFg, absFg, "absolute date must be visually distinct (dimmer) from the relative time")
}

// Scrolling to the bottom must keep the last comment reachable AND leave an
// obvious blank buffer (>= screenY/3) so "no more content" is unmistakable.
func Test_issueView_scroll_reaches_bottom_with_buffer(t *testing.T) {
	const w, h = 80, 24
	screen := newDetailTestScreen(t, w, h)
	defer screen.Fini()
	view := NewIssueView(detailTestIssue(6), nil, jira.NewJiraApiMock(nil)).(*issueView)
	view.Resize(w, h)

	view.scrollY = view.maxScrollY
	rows := renderVisibleRows(view, screen)
	joined := strings.Join(rows, "\n")

	assert.Contains(t, joined, "COMMENT-MARKER-6",
		"last comment must be reachable at maxScrollY")

	blankTrailing := 0
	for y := h - 3; y >= 0; y-- { // exclude the 2-row bottom action bar
		if strings.TrimSpace(rows[y]) == "" {
			blankTrailing++
		} else {
			break
		}
	}
	assert.GreaterOrEqual(t, blankTrailing, h/3,
		"scrolling to the bottom should leave >= screenY/3 blank buffer rows")
}
