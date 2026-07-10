package issues

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/mk-5/fjira/internal/app"
	"github.com/mk-5/fjira/internal/jira"
	"github.com/stretchr/testify/assert"
)

func mkIssue(key, summary, status, itype, assignee, updated string) jira.Issue {
	var i jira.Issue
	i.Key = key
	i.Fields.Summary = summary
	i.Fields.Status.Name = status
	i.Fields.Type.Name = itype
	i.Fields.Assignee.DisplayName = assignee
	i.Fields.Updated = updated
	return i
}

// The matchable ranges must cover only the key and summary text — never the
// type, status, assignee, or date columns.
func Test_FormatJiraIssuesWithRanges_scopesToKeyAndSummary(t *testing.T) {
	issues := []jira.Issue{
		mkIssue("PROJ-53", "Fix login flow", "Open", "Bug", "Alice", ""),
	}
	rows, ranges := FormatJiraIssuesWithRanges(issues)
	assert.Len(t, rows, 1)
	assert.Len(t, ranges, 1)
	assert.Len(t, ranges[0], 2, "expected key + summary ranges")

	row := rows[0]
	// Reconstruct the matched substrings from the byte ranges.
	keyRange := ranges[0][0]
	summaryRange := ranges[0][1]
	assert.Equal(t, "PROJ-53", row[keyRange.Start:keyRange.End])
	assert.Equal(t, "Fix login flow", row[summaryRange.Start:summaryRange.End])

	// The columns that must NOT be matchable ("BUG", "Alice") fall outside both
	// ranges.
	inRange := func(idx int) bool {
		for _, r := range ranges[0] {
			if idx >= r.Start && idx < r.End {
				return true
			}
		}
		return false
	}
	for _, col := range []string{"BUG", "Alice", "OPEN"} {
		// find the column substring in the row and assert none of its bytes are matchable
		for start := 0; start+len(col) <= len(row); start++ {
			if row[start:start+len(col)] == col {
				for i := start; i < start+len(col); i++ {
					assert.Falsef(t, inRange(i), "column %q byte %d should not be matchable", col, i)
				}
			}
		}
	}
}

// End-to-end: matching happens against key+summary only, highlighted indices
// land inside those ranges, and match.Index still identifies the right issue
// even when fuzzy scoring reorders the results.
func Test_rangeProvider_highlightAndSelection(t *testing.T) {
	issues := []jira.Issue{
		mkIssue("AAA-1", "unrelated ticket", "Open", "Task", "Bob", ""),
		mkIssue("AAA-2", "login page bug", "Open", "Bug", "Alice", ""),
	}
	rows, ranges := FormatJiraIssuesWithRanges(issues)

	screen := tcell.NewSimulationScreen("utf-8")
	_ = screen.Init() //nolint:errcheck
	defer screen.Fini()
	app.CreateNewAppWithScreen(screen)

	provider := func(q string) ([]string, [][]app.MatchRange) { return rows, ranges }
	ff := app.NewFuzzyFindWithRangeProvider("t", provider)
	ff.SetDebounceDisabled(true)
	ff.SetQuery("login")
	ff.ForceUpdate()

	matches := ff.Matches()
	assert.NotEmpty(t, matches)
	top := matches[0]

	// The top match must be AAA-2 (its summary contains "login"), proving
	// match.Index maps back to the right issue after scoring.
	assert.Equal(t, "AAA-2", issues[top.Index].Key)

	// Every highlighted byte index falls within a matchable range of that row.
	inRange := func(idx int) bool {
		for _, r := range ranges[top.Index] {
			if idx >= r.Start && idx < r.End {
				return true
			}
		}
		return false
	}
	assert.NotEmpty(t, top.MatchedIndexes)
	for _, idx := range top.MatchedIndexes {
		assert.Truef(t, inRange(idx), "highlighted index %d outside key/summary ranges", idx)
	}
}
