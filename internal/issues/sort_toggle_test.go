package issues

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/mk-5/fjira/internal/app"
	"github.com/mk-5/fjira/internal/jira"
	"github.com/mk-5/fjira/internal/ui"
	"github.com/stretchr/testify/assert"
)

func Test_currentOrderBy(t *testing.T) {
	defer func() { sortByUpdated = false }()

	sortByUpdated = false
	assert.Equal(t, OrderByStatus, currentOrderBy())

	sortByUpdated = true
	assert.Equal(t, OrderByUpdated, currentOrderBy())
}

func Test_searchIssuesView_toggleSortUpdatesBarLabel(t *testing.T) {
	screen := tcell.NewSimulationScreen("utf-8")
	_ = screen.Init() //nolint:errcheck
	defer screen.Fini()
	app.InitTestApp(screen)
	defer func() { sortByUpdated = false }()

	sortByUpdated = false
	view := NewIssuesSearchView(&jira.Project{Id: "TEST", Key: "TEST", Name: "TEST"}, nil, jira.NewJiraApiMock(nil)).(*searchIssuesView)

	// initial label reflects the default (status) sort
	item := view.bottomBar.GetItemById(int(ui.ActionToggleSort))
	assert.NotNil(t, item)
	assert.Equal(t, ui.MessageSortByStatus, item.Text1)

	// flipping the global and syncing the bar item switches the label
	sortByUpdated = true
	view.updateSortBarItem()
	assert.Equal(t, ui.MessageSortByUpdated, view.bottomBar.GetItemById(int(ui.ActionToggleSort)).Text1)

	// and back
	sortByUpdated = false
	view.updateSortBarItem()
	assert.Equal(t, ui.MessageSortByStatus, view.bottomBar.GetItemById(int(ui.ActionToggleSort)).Text1)
}

func Test_searchIssuesView_clearAllFilters(t *testing.T) {
	screen := tcell.NewSimulationScreen("utf-8")
	_ = screen.Init() //nolint:errcheck
	defer screen.Fini()
	app.InitTestApp(screen)
	defer resetFilterGlobals()

	// given every filter is set
	searchForStatus = &jira.IssueStatus{Id: "10", Name: "In Progress"}
	searchForUser = &jira.User{AccountId: "acc-1", DisplayName: "Jane Doe"}
	searchForLabel = "backend"
	excludedStatuses = []*jira.IssueStatus{{Id: "20", Name: "Done"}}
	view := NewIssuesSearchView(&jira.Project{Id: "TEST", Key: "TEST", Name: "TEST"}, nil, jira.NewJiraApiMock(nil)).(*searchIssuesView)
	view.Resize(120, 40)

	// the top-bar labels reflect the set filters after an Update pass
	view.Update()
	assert.Equal(t, "In Progress", view.topBar.GetItem(topBarStatus).Text2)
	assert.Equal(t, "Jane Doe", view.topBar.GetItem(topBarAssignee).Text2)
	assert.Equal(t, "backend", view.topBar.GetItem(topBarLabel).Text2)

	// when
	view.clearAllFilters()

	// then all four globals are cleared
	assert.Nil(t, searchForStatus)
	assert.Nil(t, searchForUser)
	assert.Equal(t, "", searchForLabel)
	assert.Nil(t, excludedStatuses)

	// and the top-bar labels reset to "All" (the else-branches in Update) rather
	// than showing stale filter values
	view.Update()
	assert.Equal(t, ui.MessageAll, view.topBar.GetItem(topBarStatus).Text2)
	assert.Equal(t, ui.MessageAll, view.topBar.GetItem(topBarAssignee).Text2)
	assert.Equal(t, ui.MessageAll, view.topBar.GetItem(topBarLabel).Text2)
}

func Test_searchIssuesView_reopenPreservesSortLabel(t *testing.T) {
	screen := tcell.NewSimulationScreen("utf-8")
	_ = screen.Init() //nolint:errcheck
	defer screen.Fini()
	app.InitTestApp(screen)
	defer func() { sortByUpdated = false }()

	// A view recreated (as reopen() does) while sortByUpdated is set must show
	// the updated label, not reset to the default.
	sortByUpdated = true
	view := NewIssuesSearchView(&jira.Project{Id: "TEST", Key: "TEST", Name: "TEST"}, nil, jira.NewJiraApiMock(nil)).(*searchIssuesView)
	item := view.bottomBar.GetItemById(int(ui.ActionToggleSort))
	assert.NotNil(t, item)
	assert.Equal(t, ui.MessageSortByUpdated, item.Text1)
}
