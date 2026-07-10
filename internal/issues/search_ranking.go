package issues

import "github.com/mk-5/fjira/internal/jira"

// issueMatchesFilters reports whether an issue satisfies the currently active
// by-status / by-assignee / by-label filters. A nil/empty filter is treated as
// "no constraint". Used during search to float filter-aligned issues to the top
// as a soft tiebreak (the server query itself is unfiltered so all matches are
// returned).
func issueMatchesFilters(issue *jira.Issue, status *jira.IssueStatus, user *jira.User, label string) bool {
	if status != nil && status.Id != "" && issue.Fields.Status.Id != status.Id {
		return false
	}
	if user != nil {
		want := user.AccountId
		got := issue.Fields.Assignee.AccountId
		if want == "" {
			want = user.DisplayName
			got = issue.Fields.Assignee.DisplayName
		}
		if want != "" && got != want {
			return false
		}
	}
	if label != "" {
		found := false
		for _, l := range issue.Fields.Labels {
			if l == label {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// orderAlignedFirst returns a new slice with filter-aligned issues first,
// preserving the original relative order within the aligned and non-aligned
// groups. This is a stable partition, so it composes cleanly as a tiebreak
// under the fuzzy finder's stable sort.
func orderAlignedFirst(issues []jira.Issue, status *jira.IssueStatus, user *jira.User, label string) []jira.Issue {
	aligned := make([]jira.Issue, 0, len(issues))
	rest := make([]jira.Issue, 0, len(issues))
	for _, issue := range issues {
		if issueMatchesFilters(&issue, status, user, label) {
			aligned = append(aligned, issue)
		} else {
			rest = append(rest, issue)
		}
	}
	return append(aligned, rest...)
}

// issueHasExcludedStatus reports whether the issue's status is in the excluded
// set. Excluded issues are shown dimmed and sorted last during search.
func issueHasExcludedStatus(issue *jira.Issue, excluded []*jira.IssueStatus) bool {
	for _, es := range excluded {
		if es != nil && es.Id != "" && issue.Fields.Status.Id == es.Id {
			return true
		}
	}
	return false
}
