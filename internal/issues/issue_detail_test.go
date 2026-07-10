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

func Test_formatDetailRows_aligns_labels(t *testing.T) {
	rows := formatDetailRows([]detailRow{
		{"Priority", "High"},
		{"Type", "Bug"},
		{"Created", "3 days ago"},
	})
	// Every value column must start at the same offset (widest label + 2).
	firstColon := -1
	for _, r := range rows {
		idx := strings.Index(r, " High")
		if idx < 0 {
			idx = strings.Index(r, " Bug")
		}
		if idx < 0 {
			idx = strings.Index(r, " 3 days ago")
		}
		if firstColon == -1 {
			firstColon = idx
		}
		assert.Equal(t, firstColon, idx, "value columns should be left-aligned to the same offset in row %q", r)
	}
	// A blank value renders without panicking and keeps the row.
	rows = formatDetailRows([]detailRow{{"Priority", ""}})
	assert.Len(t, rows, 1)
	assert.Contains(t, rows[0], "Priority")
}

func Test_buildDetailRows_content(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	issue := &jira.Issue{}
	issue.Fields.Priority.Name = "High"
	issue.Fields.Type.Name = "Bug"
	issue.Fields.Created = "2026-07-07T12:00:00.000+0000"
	issue.Fields.Updated = "2026-07-10T10:00:00.000+0000"
	rows := buildDetailRows(issue, now)
	assert.Equal(t, []detailRow{
		{ui.MessageDetailPriority, "High"},
		{ui.MessageDetailType, "Bug"},
		{ui.MessageDetailCreated, "3 days ago"},
		{ui.MessageDetailUpdated, "2 hours ago"},
	}, rows)
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
