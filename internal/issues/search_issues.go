package issues

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/mk-5/fjira/internal/app"
	"github.com/mk-5/fjira/internal/boards"
	"github.com/mk-5/fjira/internal/jira"
	"github.com/mk-5/fjira/internal/statuses"
	"github.com/mk-5/fjira/internal/ui"
	"github.com/mk-5/fjira/internal/users"
)

type searchIssuesView struct {
	api          jira.Api
	bottomBar    *app.ActionBar
	topBar       *app.ActionBar
	fuzzyFind    *app.FuzzyFind
	project      *jira.Project
	goBackFn     func()
	currentQuery string
	customJql    string
	screenX      int
	screenY      int
	issues       []jira.Issue
	labels       []string
	dirty        bool // refetch jira issues from api if dirty
	// cachedBoards memoizes FindBoards() for the lifetime of this view.
	// FindBoards paginates sequentially against /rest/agile/1.0/board (Cloud
	// caps page size at 50), so a project with hundreds of boards costs
	// multiple round-trips on every F4 press. Boards rarely change during a
	// session; cache miss = first F4 press, hit = every subsequent press.
	cachedBoards []jira.BoardItem
}

const (
	JiraFetchRecordsThreshold = 100
	topBarStatus              = 1
	topBarExcludeStatus       = 2
	topBarAssignee            = 3
	topBarLabel               = 4
)

var (
	issueRegExp            = regexp.MustCompile("^[A-Za-z0-9]{2,10}-[0-9]+$")
	issueRegExpOnlyNumeric = regexp.MustCompile("^[0-9]+$")
	searchForStatus        *jira.IssueStatus // global in order to keep status&user between views
	searchForUser          *jira.User
	searchForLabel         string
	excludedStatuses       []*jira.IssueStatus
	// sortByUpdated toggles the issue list order between status (default) and
	// last-updated ascending. Global, like the filters above, so it survives
	// the view recreation reopen() does. In-session only — not persisted.
	sortByUpdated  bool
	searchNavItems = []ui.NavItemConfig{
		ui.NavItemConfig{Action: ui.ActionSearchByStatus, Text1: ui.MessageByStatus, Text2: "[F1]", Key: tcell.KeyF1},
		ui.NavItemConfig{Action: ui.ActionSearchByAssignee, Text1: ui.MessageByAssignee, Text2: "[F2]", Key: tcell.KeyF2},
		ui.NavItemConfig{Action: ui.ActionSearchByLabel, Text1: ui.MessageByLabel, Text2: "[F3]", Key: tcell.KeyF3},
		ui.NavItemConfig{Action: ui.ActionBoards, Text1: ui.MessageBoards, Text2: "[F4]", Key: tcell.KeyF4},
		ui.NavItemConfig{Action: ui.ActionCreateIssue, Text1: ui.MessageCreateIssue, Text2: "[F6]", Key: tcell.KeyF6},
		ui.NavItemConfig{Action: ui.ActionExcludeStatus, Text1: ui.MessageExcludeStatus, Text2: "[F7]", Key: tcell.KeyF7},
	}
)

func NewIssuesSearchView(project *jira.Project, goBackFn func(), api jira.Api) app.View {
	bottomBar := ui.CreateBottomActionBarWithItems(searchNavItems)
	// F8 clear-filters is always shown (clearing when nothing is set is a
	// no-op) so it keeps a stable position ahead of F9. ActionBar has no
	// insert-at-index, so both are added at construction in the desired order.
	bottomBar.AddItem(ui.NewClearFiltersBarItem())
	// F9 sort toggle. Initial label reflects the sort-mode global so it stays
	// correct across reopen() view recreations, not just the first launch.
	sortLabel := ui.MessageSortByStatus
	if sortByUpdated {
		sortLabel = ui.MessageSortByUpdated
	}
	bottomBar.AddItem(ui.NewToggleSortBarItem(sortLabel))
	topBarItems := []ui.NavItemConfig{
		ui.NavItemConfig{Text1: ui.MessageProjectLabel, Text2: app.ActionBarLabel(fmt.Sprintf("[%s]%s", project.Key, project.Name))},
		ui.NavItemConfig{Text1: ui.MessageLabelStatus, Text2: ui.MessageAll},
		ui.NavItemConfig{Text1: "Exclude Status: ", Text2: "-"},
		ui.NavItemConfig{Text1: ui.MessageLabelAssignee, Text2: ui.MessageAll},
		ui.NavItemConfig{Text1: ui.MessageLabelLabel, Text2: ui.MessageAll},
	}
	topBar := ui.CreateTopActionBarWithItems(topBarItems)
	return &searchIssuesView{
		api:       api,
		goBackFn:  goBackFn,
		bottomBar: bottomBar,
		topBar:    topBar,
		project:   project,
	}
}

func NewIssuesSearchViewWithCustomJql(jql string, goBackFn func(), api jira.Api) app.View {
	project := &jira.Project{Id: "", Key: ui.MessageCustomJql, Name: ""}
	var jqlTopBar string
	jqlTopBar = jql
	if len(jqlTopBar) > 250 {
		jqlTopBar = jqlTopBar[:250]
	}
	topBar := ui.CreateTopActionBarWithItems([]ui.NavItemConfig{
		ui.NavItemConfig{Text1: ui.MessageJqlLabel, Text2: app.ActionBarLabel(jqlTopBar)},
	})
	return &searchIssuesView{
		api:       api,
		goBackFn:  goBackFn,
		bottomBar: app.NewActionBar(app.Bottom, app.Left),
		topBar:    topBar,
		project:   project,
		customJql: jql,
	}
}

func (view *searchIssuesView) Init() {
	app.GetApp().LoadingWithText(true, ui.MessageSearchIssuesLoading)
	if view.project.Id == ui.MessageAll {
		view.bottomBar.RemoveItem(int(ui.ActionSearchByStatus))
		view.bottomBar.RemoveItem(int(ui.ActionSearchByAssignee))
	}
	// Restore the by-status/assignee/label/excluded filters the user last
	// viewed for this project (load-or-clear, so another project's filters
	// never leak in). Runs before the first query so JQL and top bar reflect
	// the restored state immediately.
	restoreFilters(view.project)
	go view.runIssuesFuzzyFind()
	go view.handleSearchActions()
}

func (view *searchIssuesView) Destroy() {
	// do nothing
}

func (view *searchIssuesView) Draw(screen tcell.Screen) {
	view.bottomBar.Draw(screen)
	if view.fuzzyFind != nil {
		view.fuzzyFind.Draw(screen)
	}
	view.topBar.Draw(screen)
}

func (view *searchIssuesView) Update() {
	view.bottomBar.Update()
	view.topBar.Update()
	if view.fuzzyFind != nil {
		view.fuzzyFind.Update()
	}
	// Each label follows its global: show the value when set, otherwise reset to
	// "All". The else-branches matter for clear-filters — without them the bar
	// would keep showing stale values after the globals are nil'd.
	statusText := ui.MessageAll
	if searchForStatus != nil {
		statusText = searchForStatus.Name
	}
	if view.topBar.GetItem(topBarStatus).Text2 != statusText {
		view.topBar.GetItem(topBarStatus).ChangeText(ui.MessageLabelStatus, statusText)
		view.topBar.Resize(view.screenX, view.screenY)
	}
	assigneeText := ui.MessageAll
	if searchForUser != nil {
		assigneeText = searchForUser.DisplayName
	}
	if view.topBar.GetItem(topBarAssignee).Text2 != assigneeText {
		view.topBar.GetItem(topBarAssignee).ChangeText(ui.MessageLabelAssignee, assigneeText)
		view.topBar.Resize(view.screenX, view.screenY)
	}
	labelText := ui.MessageAll
	if searchForLabel != "" {
		labelText = searchForLabel
	}
	if view.topBar.GetItem(topBarLabel).Text2 != labelText {
		view.topBar.GetItem(topBarLabel).ChangeText(ui.MessageLabelLabel, labelText)
		view.topBar.Resize(view.screenX, view.screenY)
	}
	view.refreshExcludedStatusesUI()
}

// refreshExcludedStatusesUI keeps the top-bar "Exclude Status: " text in sync
// with the excludedStatuses global. The F8 clear-filters button is now always
// visible, so it is no longer added/removed here.
func (view *searchIssuesView) refreshExcludedStatusesUI() {
	if len(excludedStatuses) > 0 {
		names := make([]string, len(excludedStatuses))
		for i, s := range excludedStatuses {
			names[i] = s.Name
		}
		expected := fmt.Sprintf("-%s", strings.Join(names, ", -"))
		if view.topBar.GetItem(topBarExcludeStatus).Text2 != expected {
			view.topBar.GetItem(topBarExcludeStatus).ChangeText("Exclude Status: ", expected)
			view.topBar.Resize(view.screenX, view.screenY)
		}
		return
	}
	if view.topBar.GetItem(topBarExcludeStatus).Text2 != "-" {
		view.topBar.GetItem(topBarExcludeStatus).ChangeText("Exclude Status: ", "-")
		view.topBar.Resize(view.screenX, view.screenY)
	}
}

func (view *searchIssuesView) Resize(screenX, screenY int) {
	view.bottomBar.Resize(screenX, screenY)
	view.topBar.Resize(screenX, screenY)
	if view.fuzzyFind != nil {
		view.fuzzyFind.Resize(screenX, screenY)
	}
	view.screenX = screenX
	view.screenY = screenY
}

func (view *searchIssuesView) HandleKeyEvent(ev *tcell.EventKey) {
	// Inline call (was `go view.bottomBar.HandleKeyEvent(ev)` per the TODO):
	// safe now that ActionBar.Action is buffered. The previous goroutine
	// spawn-per-keystroke was a goroutine leak — pprof on a running
	// session showed multiple ActionBar.HandleKeyEvent goroutines stuck on
	// `chan send` for 60+ minutes.
	view.bottomBar.HandleKeyEvent(ev)
	if view.fuzzyFind != nil {
		view.fuzzyFind.HandleKeyEvent(ev)
	}
}

func (view *searchIssuesView) runIssuesFuzzyFind() {
	a := app.GetApp()
	view.fuzzyFind = app.NewFuzzyFindWithRangeProvider(ui.MessageSelectIssue, view.findIssuesWithRanges)
	view.fuzzyFind.MarginBottom = 1
	if view.customJql != "" {
		view.fuzzyFind.MarginBottom = 0
	}
	// Esc here should clear a non-empty query rather than back out to the
	// projects list — typo-correction is the common case; abandoning the
	// project is the rare one. Second Esc on an empty query still exits.
	view.fuzzyFind.SetClearOnEsc(true)
	a.Loading(false)
	a.ClearNow()
	if chosen := <-view.fuzzyFind.Complete; true {
		a.ClearNow()
		if chosen.Index < 0 {
			view.goBack()
			searchForStatus = nil
			searchForUser = nil
			excludedStatuses = nil
			return
		}
		chosenIssue := view.issues[chosen.Index]
		go view.goToIssueView(chosenIssue.Key)
	}
}

func (view *searchIssuesView) goToIssueView(issueKey string) {
	app.GoTo("issue", issueKey, view.reopen, view.api)
}

// refetchIfNeeded fetches issues from the API when the current cache is stale
// for the given query, and records the query. Shared by the fuzzy-find
// providers so their fetch semantics stay identical.
func (view *searchIssuesView) refetchIfNeeded(query string) {
	a := app.GetApp()
	query = strings.TrimSpace(query)

	// when no custom jql set
	// when manual set dirty=true
	// when there is more records than max
	// when query has issue format (e.g. PROJ-1234)
	// when query is numeric-only and a project is selected (we'll expand
	//   it to PROJ-N server-side so an exact issue-key match can be found)
	// when there is no results
	if view.customJql == "" || len(view.issues) >= JiraFetchRecordsThreshold || len(view.issues) == 0 || view.dirty || view.queryHasIssueFormat() || view.queryIsNumericWithProject(query) || query == "" {
		a.LoadingWithText(true, ui.MessageSearchIssuesLoading)
		view.issues = view.searchForIssues(query)
		a.Loading(false)
		view.dirty = false
	}

	view.currentQuery = query
}

// findIssuesWithRanges is the range-aware fuzzy-find provider. Besides the
// display rows and matchable key/summary ranges, it returns a per-row "dimmed"
// flag. During an active search (filters relaxed server-side) it reorders the
// fetched issues so filter-aligned ones come first — a soft tiebreak under the
// fuzzy sort — and flags excluded-status issues dimmed so they render muted and
// sort last. When browsing (no query) nothing is reordered or dimmed.
func (view *searchIssuesView) findIssuesWithRanges(query string) ([]string, [][]app.MatchRange, []bool) {
	view.refetchIfNeeded(query)
	if strings.TrimSpace(query) != "" {
		view.issues = orderAlignedFirst(view.issues, searchForStatus, searchForUser, searchForLabel)
	}
	rows, ranges := FormatJiraIssuesWithRanges(view.issues)
	dimmed := make([]bool, len(view.issues))
	for i := range view.issues {
		dimmed[i] = issueHasExcludedStatus(&view.issues[i], excludedStatuses)
	}
	return rows, ranges, dimmed
}

// queryIsNumericWithProject reports whether the given query is purely numeric
// AND a non-"All" project is currently selected — the conditions under which
// searchForIssues expands `1234` to `PROJ-1234`.
func (view *searchIssuesView) queryIsNumericWithProject(query string) bool {
	if query == "" || view.project == nil || view.project.Key == "" || view.project.Key == ui.MessageAll {
		return false
	}
	return issueRegExpOnlyNumeric.MatchString(query)
}

func (view *searchIssuesView) handleSearchActions() {
	if selectedAction := <-view.bottomBar.Action; true {
		switch selectedAction {
		case ui.ActionSearchByStatus:
			view.runSelectStatus()
		case ui.ActionSearchByAssignee:
			view.runSelectUser()
		case ui.ActionSearchByLabel:
			view.runSelectLabel()
		case ui.ActionBoards:
			view.runSelectBoard()
		case ui.ActionCreateIssue:
			projectId := ""
			if view.project != nil {
				projectId = view.project.Id
			}
			ui.OpenCreateIssueInBrowser(view.api, projectId, 0)
			go view.runIssuesFuzzyFind()
			go view.handleSearchActions()
		case ui.ActionExcludeStatus:
			view.runExcludeStatus()
		case ui.ActionClearFilters:
			view.clearAllFilters()
		case ui.ActionToggleSort:
			view.runToggleSort()
		}
	}
}

// currentOrderBy maps the sort-mode global to its JQL ORDER BY clause.
func currentOrderBy() string {
	if sortByUpdated {
		return OrderByUpdated
	}
	return OrderByStatus
}

// runToggleSort flips the list order between status and last-updated, updates
// the F9 bar label, and refetches so the new ORDER BY takes effect. Mirrors the
// other mutation handlers: mutate global, mark dirty, re-run the fuzzy find.
func (view *searchIssuesView) runToggleSort() {
	sortByUpdated = !sortByUpdated
	view.updateSortBarItem()
	view.dirty = true
	go view.runIssuesFuzzyFind()
	go view.handleSearchActions()
}

// updateSortBarItem keeps the F9 bar label in sync with the sort-mode global.
// Looks the item up by id at call time — GetItem is positional and the pointer
// would drift as the dynamic F8 clear item is added/removed.
func (view *searchIssuesView) updateSortBarItem() {
	item := view.bottomBar.GetItemById(int(ui.ActionToggleSort))
	if item == nil {
		return
	}
	label := ui.MessageSortByStatus
	if sortByUpdated {
		label = ui.MessageSortByUpdated
	}
	item.ChangeText(label, "[F9]")
	// Re-layout so the new label width shifts sibling items correctly, matching
	// the top-bar filter-label update sites.
	view.bottomBar.Resize(view.screenX, view.screenY)
}

func (view *searchIssuesView) runSelectStatus() {
	app.GetApp().ClearNow()
	app.GetApp().Loading(true)
	ss := view.fetchStatuses(view.project.Id)
	ss = append(ss, jira.IssueStatus{Name: ui.MessageAll})
	statusesStrings := statuses.FormatJiraStatuses(ss)
	view.fuzzyFind = app.NewFuzzyFind(ui.MessageStatusFuzzyFind, statusesStrings)
	app.GetApp().Loading(false)
	if status := <-view.fuzzyFind.Complete; true {
		app.GetApp().ClearNow()
		if status.Index >= 0 && len(ss) > 0 {
			searchForStatus = &ss[status.Index]
			view.dirty = true
			saveFilters(view.project)
		}
		go view.runIssuesFuzzyFind()
		go view.handleSearchActions()
	}
}

func (view *searchIssuesView) runExcludeStatus() {
	app.GetApp().ClearNow()
	app.GetApp().Loading(true)
	ss := view.fetchStatuses(view.project.Id)
	ss = append(ss, jira.IssueStatus{Name: ui.MessageAll})
	statusesStrings := statuses.FormatJiraStatuses(ss)
	view.fuzzyFind = app.NewFuzzyFind("Select status to exclude or ESC to cancel", statusesStrings)
	app.GetApp().Loading(false)
	if status := <-view.fuzzyFind.Complete; true {
		app.GetApp().ClearNow()
		if status.Index >= 0 && len(ss) > 0 {
			selected := &ss[status.Index]
			if selected.Name != ui.MessageAll {
				dup := false
				for _, e := range excludedStatuses {
					if e.Id == selected.Id {
						dup = true
						break
					}
				}
				if !dup {
					excludedStatuses = append(excludedStatuses, selected)
					view.dirty = true
					saveFilters(view.project)
				}
			}
		}
		go view.runIssuesFuzzyFind()
		go view.handleSearchActions()
	}
}

// clearAllFilters (F8) resets every filter — status, assignee, label, and
// excluded statuses — then refetches. Unlike the Esc-reset, this is an explicit
// user action, so the emptied state is persisted per-project via saveFilters.
// The top-bar labels reset to "All" on the next Update() (see the else-branches
// there).
func (view *searchIssuesView) clearAllFilters() {
	searchForStatus = nil
	searchForUser = nil
	searchForLabel = ""
	excludedStatuses = nil
	view.dirty = true
	saveFilters(view.project)
	go view.runIssuesFuzzyFind()
	go view.handleSearchActions()
}

func (view *searchIssuesView) runSelectUser() {
	app.GetApp().ClearNow()
	app.GetApp().Loading(true)
	var us *[]jira.User
	view.fuzzyFind, us = users.NewFuzzyFind(view.project.Key, view.api)
	app.GetApp().Loading(false)
	if user := <-view.fuzzyFind.Complete; true {
		app.GetApp().ClearNow()
		if user.Index >= 0 && len(*us) > 0 {
			searchForUser = &(*us)[user.Index]
			view.dirty = true
			saveFilters(view.project)
		}
		go view.runIssuesFuzzyFind()
		go view.handleSearchActions()
	}
}

func (view *searchIssuesView) runSelectLabel() {
	app.GetApp().ClearNow()
	app.GetApp().Loading(true)
	view.fuzzyFind = app.NewFuzzyFindWithProvider(ui.MessageSelectLabel, view.findLabels)
	app.GetApp().Loading(false)
	if label := <-view.fuzzyFind.Complete; true {
		app.GetApp().ClearNow()
		if label.Index >= 0 && len(view.labels) > 0 {
			searchForLabel = view.labels[label.Index]
			view.dirty = true
			saveFilters(view.project)
		}
		go view.runIssuesFuzzyFind()
		go view.handleSearchActions()
	}
}

func (view *searchIssuesView) runSelectBoard() {
	app.GetApp().ClearNow()
	app.GetApp().Loading(true)
	bs := view.findBoards()
	boardsString := boards.FormatJiraBoards(bs)
	view.fuzzyFind = app.NewFuzzyFind(ui.MessageSelectBoard, boardsString)
	app.GetApp().Loading(false)
	if board := <-view.fuzzyFind.Complete; true {
		app.GetApp().ClearNow()
		if board.Index >= 0 && len(boardsString) > 0 {
			app.GoTo("boards", view.project, &bs[board.Index], view.reopen, view.api)
			return
		}
		go view.runIssuesFuzzyFind()
		go view.handleSearchActions()
	}
}

func (view *searchIssuesView) reopen() {
	if view.customJql != "" {
		app.GoTo("issues-search-jql", view.customJql, view.goBackFn, view.api)
		return
	}
	app.GoTo("issues-search", view.project.Id, view.goBackFn, view.api)
}

func (view *searchIssuesView) searchForIssues(query string) []jira.Issue {
	q := strings.TrimSpace(query)
	var jql string
	switch {
	case view.customJql != "":
		// Custom JQL mode bypasses query construction entirely.
		jql = view.customJql
	case view.queryIsNumericWithProject(q):
		// Numeric query + project selected: prefix-match issue keys.
		// `53` against COINS matches COINS-53, COINS-537, COINS-5300, etc.
		// JQL's `key ~ "PROJ-N*"` does a real key prefix match (not a
		// summary text match), so `53` won't pick up COINS-153 by accident.
		jql = fmt.Sprintf(`project=%s AND key ~ "%s-%s*" ORDER BY key DESC`, view.project.Id, view.project.Key, q)
	case q != "":
		// Active text search: drop every filter (status/assignee/label/excluded)
		// from the server query so any project issue matching the term is
		// returned — "find anything the JQL would return". Filter semantics move
		// client-side: filter-aligned issues float up (input-order tiebreak under
		// the fuzzy sort) and excluded-status issues are shown dimly and last.
		jql = BuildSearchIssuesJql(view.project, q, nil, nil, "", nil, currentOrderBy())
	default:
		// No query = browsing: filters are a hard intersection.
		jql = BuildSearchIssuesJql(view.project, q, searchForStatus, searchForUser, searchForLabel, excludedStatuses, currentOrderBy())
	}
	issues, err := view.api.SearchJql(jql)
	if err != nil {
		app.Error(err.Error())
	}
	return issues
}

func (view *searchIssuesView) fetchStatuses(projectId string) []jira.IssueStatus {
	app.GetApp().Loading(true)
	ss, err := view.api.FindProjectStatuses(projectId)
	if err != nil {
		app.Error(err.Error())
	}
	app.GetApp().Loading(false)
	return ss
}

func (view *searchIssuesView) findLabels(query string) []string {
	app.GetApp().LoadingWithText(true, ui.MessageSearchLabelsLoading)
	labels, err := view.api.FindLabels(nil, query)
	labels = append(labels, ui.MessageAll)
	app.GetApp().Loading(false)
	if err != nil {
		app.Error(err.Error())
	}
	view.labels = labels
	return labels
}

func (view *searchIssuesView) findBoards() []jira.BoardItem {
	if view.cachedBoards != nil {
		return view.cachedBoards
	}
	app.GetApp().LoadingWithText(true, ui.MessageSearchBoardsLoading)
	bs, err := view.api.FindBoards(view.project.Id)
	app.GetApp().Loading(false)
	if err != nil {
		app.Error(err.Error())
		return bs
	}
	view.cachedBoards = bs
	return bs
}

func (view *searchIssuesView) queryHasIssueFormat() bool {
	return issueRegExp.MatchString(view.currentQuery)
}

func (view *searchIssuesView) goBack() {
	if view.goBackFn != nil {
		view.goBackFn()
	}
}
