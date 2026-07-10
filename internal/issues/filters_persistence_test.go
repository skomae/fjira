package issues

import (
	"testing"

	"github.com/mk-5/fjira/internal/jira"
	os2 "github.com/mk-5/fjira/internal/os"
	"github.com/stretchr/testify/assert"
)

func Test_saveAndRestoreFilters_roundTrip(t *testing.T) {
	// given - an isolated home so the round-trip hits a throwaway fjira.yaml
	_ = os2.SetUserHomeDir(t.TempDir())
	defer resetFilterGlobals()
	project := &jira.Project{Id: "COINS", Key: "COINS", Name: "Coins"}
	searchForStatus = &jira.IssueStatus{Id: "10", Name: "In Progress"}
	searchForUser = &jira.User{AccountId: "acc-1", DisplayName: "Jane Doe"}
	searchForLabel = "backend"
	excludedStatuses = []*jira.IssueStatus{
		{Id: "20", Name: "Done"},
		{Id: "30", Name: "Rejected"},
	}

	// when - persist, wipe live state, then restore
	saveFilters(project)
	resetFilterGlobals()
	restoreFilters(project)

	// then - every filter is reconstructed, excluded ids/names stay index-aligned
	assert.NotNil(t, searchForStatus)
	assert.Equal(t, "10", searchForStatus.Id)
	assert.Equal(t, "In Progress", searchForStatus.Name)
	assert.NotNil(t, searchForUser)
	assert.Equal(t, "acc-1", searchForUser.AccountId)
	assert.Equal(t, "Jane Doe", searchForUser.DisplayName)
	assert.Equal(t, "backend", searchForLabel)
	assert.Len(t, excludedStatuses, 2)
	assert.Equal(t, "20", excludedStatuses[0].Id)
	assert.Equal(t, "Done", excludedStatuses[0].Name)
	assert.Equal(t, "30", excludedStatuses[1].Id)
	assert.Equal(t, "Rejected", excludedStatuses[1].Name)
}

func Test_restoreFilters_unknownProjectClears(t *testing.T) {
	// given - COINS has saved filters, but we restore a never-saved project
	_ = os2.SetUserHomeDir(t.TempDir())
	defer resetFilterGlobals()
	coins := &jira.Project{Id: "COINS", Key: "COINS", Name: "Coins"}
	searchForStatus = &jira.IssueStatus{Id: "10", Name: "In Progress"}
	searchForUser = &jira.User{AccountId: "acc-1", DisplayName: "Jane Doe"}
	searchForLabel = "backend"
	excludedStatuses = []*jira.IssueStatus{{Id: "20", Name: "Done"}}
	saveFilters(coins)

	// when - switching to a project with no saved filters (COINS's state still live)
	other := &jira.Project{Id: "FOO", Key: "FOO", Name: "Foo"}
	restoreFilters(other)

	// then - load-or-clear: no leak from COINS
	assert.Nil(t, searchForStatus)
	assert.Nil(t, searchForUser)
	assert.Equal(t, "", searchForLabel)
	assert.Nil(t, excludedStatuses)
}

func resetFilterGlobals() {
	searchForStatus = nil
	searchForUser = nil
	searchForLabel = ""
	excludedStatuses = nil
}
