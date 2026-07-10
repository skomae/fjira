package issues

import (
	"net/http"
	"testing"

	"github.com/mk-5/fjira/internal/jira"
	"github.com/stretchr/testify/assert"
)

func issueWith(key, statusId, assigneeAcc string, labels ...string) jira.Issue {
	var i jira.Issue
	i.Key = key
	i.Fields.Status.Id = statusId
	i.Fields.Assignee.AccountId = assigneeAcc
	i.Fields.Labels = labels
	return i
}

func Test_issueMatchesFilters(t *testing.T) {
	i := issueWith("A-1", "10", "acc-1", "backend")

	assert.True(t, issueMatchesFilters(&i, nil, nil, ""), "no filters -> aligned")
	assert.True(t, issueMatchesFilters(&i, &jira.IssueStatus{Id: "10"}, nil, ""))
	assert.False(t, issueMatchesFilters(&i, &jira.IssueStatus{Id: "99"}, nil, ""))
	assert.True(t, issueMatchesFilters(&i, nil, &jira.User{AccountId: "acc-1"}, ""))
	assert.False(t, issueMatchesFilters(&i, nil, &jira.User{AccountId: "acc-2"}, ""))
	assert.True(t, issueMatchesFilters(&i, nil, nil, "backend"))
	assert.False(t, issueMatchesFilters(&i, nil, nil, "frontend"))
	// all filters together
	assert.True(t, issueMatchesFilters(&i, &jira.IssueStatus{Id: "10"}, &jira.User{AccountId: "acc-1"}, "backend"))
}

func Test_orderAlignedFirst_stablePartition(t *testing.T) {
	issues := []jira.Issue{
		issueWith("A-1", "10", "", ""), // aligned (status 10)
		issueWith("A-2", "99", "", ""), // not aligned
		issueWith("A-3", "10", "", ""), // aligned
		issueWith("A-4", "99", "", ""), // not aligned
	}
	out := orderAlignedFirst(issues, &jira.IssueStatus{Id: "10"}, nil, "")
	keys := []string{out[0].Key, out[1].Key, out[2].Key, out[3].Key}
	// aligned first (A-1, A-3 in original order), then the rest (A-2, A-4)
	assert.Equal(t, []string{"A-1", "A-3", "A-2", "A-4"}, keys)
}

func Test_issueHasExcludedStatus(t *testing.T) {
	i := issueWith("A-1", "done", "", "")
	assert.True(t, issueHasExcludedStatus(&i, []*jira.IssueStatus{{Id: "done"}}))
	assert.False(t, issueHasExcludedStatus(&i, []*jira.IssueStatus{{Id: "open"}}))
	assert.False(t, issueHasExcludedStatus(&i, nil))
}

// During an active text search, the server JQL must drop all filter clauses so
// any matching project issue is returned; filter semantics move client-side.
func Test_searchForIssues_dropsFiltersDuringSearch(t *testing.T) {
	defer resetFilterGlobals()
	searchForStatus = &jira.IssueStatus{Id: "10", Name: "In Progress"}
	searchForUser = &jira.User{AccountId: "acc-1", DisplayName: "Jane"}
	searchForLabel = "backend"
	excludedStatuses = []*jira.IssueStatus{{Id: "done", Name: "Done"}}

	var capturedJql string
	api := jira.NewJiraApiMock(func(w http.ResponseWriter, r *http.Request) {
		capturedJql = r.URL.Query().Get("jql")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"issues":[],"total":0,"maxResults":100,"startAt":0}`)) //nolint:errcheck
	})
	view := NewIssuesSearchView(&jira.Project{Id: "TEST", Key: "TEST", Name: "TEST"}, nil, api).(*searchIssuesView)

	view.searchForIssues("login")

	assert.Contains(t, capturedJql, "project=TEST")
	assert.Contains(t, capturedJql, `summary~"login*"`)
	assert.NotContains(t, capturedJql, "status=", "status filter must be dropped during search")
	assert.NotContains(t, capturedJql, "assignee=", "assignee filter must be dropped during search")
	assert.NotContains(t, capturedJql, "labels=", "label filter must be dropped during search")
	assert.NotContains(t, capturedJql, "status!=", "excluded-status clause must be dropped during search")
}

// When browsing (no query) the filters are a hard intersection and stay in the JQL.
func Test_searchForIssues_keepsFiltersWhenBrowsing(t *testing.T) {
	defer resetFilterGlobals()
	searchForStatus = &jira.IssueStatus{Id: "10", Name: "In Progress"}

	var capturedJql string
	api := jira.NewJiraApiMock(func(w http.ResponseWriter, r *http.Request) {
		capturedJql = r.URL.Query().Get("jql")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"issues":[],"total":0,"maxResults":100,"startAt":0}`)) //nolint:errcheck
	})
	view := NewIssuesSearchView(&jira.Project{Id: "TEST", Key: "TEST", Name: "TEST"}, nil, api).(*searchIssuesView)

	view.searchForIssues("")

	assert.Contains(t, capturedJql, "status=10", "filters apply as a hard intersection when browsing")
}
