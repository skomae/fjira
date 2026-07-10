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
	// Reconstruct the matched substrings from the byte ranges. The key range
	// covers only the numeric issue number ("53"), not the alphabetic prefix,
	// so an alphabetic query never lands on the project key.
	keyRange := ranges[0][0]
	summaryRange := ranges[0][1]
	assert.Equal(t, "53", row[keyRange.Start:keyRange.End])
	assert.Equal(t, "Fix login flow", row[summaryRange.Start:summaryRange.End])

	// The columns that must NOT be matchable fall outside both ranges. "PROJ"
	// (the alphabetic key prefix) is included: it must not be matchable either.
	inRange := func(idx int) bool {
		for _, r := range ranges[0] {
			if idx >= r.Start && idx < r.End {
				return true
			}
		}
		return false
	}
	for _, col := range []string{"BUG", "Alice", "OPEN", "PROJ"} {
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

func Test_issueKeyNumberOffset(t *testing.T) {
	assert.Equal(t, 6, issueKeyNumberOffset("COINS-115")) // points at "115"
	assert.Equal(t, 5, issueKeyNumberOffset("PROJ-53"))   // points at "53"
	// no dash, no trailing digits -> whole key matchable (offset 0)
	assert.Equal(t, 0, issueKeyNumberOffset("NODASH"))
	assert.Equal(t, 0, issueKeyNumberOffset("PROJ-abc"))
	assert.Equal(t, 0, issueKeyNumberOffset("PROJ-"))
}

// Regression for the reported bug: searching an alphabetic term ("skip")
// against COINS-115 with a title that has no clean "skip" word must NOT
// highlight any character in the issue key (the S in COINS, etc.). Digits-only
// key matching guarantees an alphabetic query can never touch the key.
func Test_rangeProvider_alphabeticQueryNeverHitsKey(t *testing.T) {
	issues := []jira.Issue{
		mkIssue("COINS-115", `C2Profile: Make "Skipping entity resolution`, "Open", "Task", "Bob", ""),
	}
	rows, ranges := FormatJiraIssuesWithRanges(issues)

	screen := tcell.NewSimulationScreen("utf-8")
	_ = screen.Init() //nolint:errcheck
	defer screen.Fini()
	app.CreateNewAppWithScreen(screen)

	provider := func(q string) ([]string, [][]app.MatchRange, []bool) {
		return rows, ranges, make([]bool, len(rows))
	}
	ff := app.NewFuzzyFindWithRangeProvider("t", provider)
	ff.SetDebounceDisabled(true)
	ff.SetQuery("skip")
	ff.ForceUpdate()

	matches := ff.Matches()
	assert.NotEmpty(t, matches)

	row := rows[0]
	keyStart := ranges[0][0].Start // numeric-suffix start
	// The alphabetic key prefix occupies bytes before keyStart. Assert no
	// highlighted index falls before the numeric suffix (i.e. nothing in the
	// "COINS-" region and nothing on the "S").
	for _, idx := range matches[0].MatchedIndexes {
		// the key column is at the very start of the row; any highlight there
		// with idx < keyStart would be on the alphabetic prefix.
		if idx < keyStart {
			assert.Failf(t, "highlight on key prefix", "index %d (%q) is in the key prefix, before the numeric suffix", idx, string(row[idx]))
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

	provider := func(q string) ([]string, [][]app.MatchRange, []bool) {
		return rows, ranges, make([]bool, len(rows))
	}
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
