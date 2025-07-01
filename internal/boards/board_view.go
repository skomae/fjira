package boards

import (
	"fmt"
	"os"
	"strings"
	"github.com/gdamore/tcell/v2"
	"github.com/mk-5/fjira/internal/app"
	"github.com/mk-5/fjira/internal/jira"
	"github.com/mk-5/fjira/internal/ui"
	"github.com/mk-5/fjira/internal/users"
)

// debugLog writes debug messages to /tmp/fjira_debug.log
func debugLog(msg string) {
	f, err := os.OpenFile("/tmp/fjira_debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		defer f.Close()
		f.WriteString(msg + "\n")
	}
}

const (
	topMargin           = 2 // 1 for navigation
	vimLeft             = 'h'
	vimDown             = 'j'
	vimUp               = 'k'
	vimRight            = 'l'
	maxIssuesNumber     = 500
	issueFetchBatchSize = 100
)

type boardView struct {
	app.View
	api                    jira.Api
	bottomBar              *app.ActionBar
	selectedIssueBottomBar *app.ActionBar
	topBar                 *app.ActionBar
	boardConfiguration     *jira.BoardConfiguration
	filterJQL              string
	project                *jira.Project
	issues                 []jira.Issue
	allIssues              []jira.Issue // stores all issues before filtering
	assigneeFilter         *jira.User   // current assignee filter
	statusesColumnsMap     map[string]int
	columnStatusesMap      map[int][]string
	columnsX               map[int]int
	issuesRow              map[string]int
	issuesColumn           map[string]int
	issuesSummaries        map[string]string
	goBackFn               func()
	columns                []string
	highlightedIssue       *jira.Issue
	issueSelected          bool
	tmpX                   int
	cursorX                int
	cursorY                int
	screenX, screenY       int
	scrollX                int
	scrollY                int
	columnSize             int
	columnHeaderStyle      tcell.Style
	issueStyle             tcell.Style
	highlightIssueStyle    tcell.Style
	selectedIssueStyle     tcell.Style
	titleStyle             tcell.Style
}

func NewBoardView(project *jira.Project, boardConfiguration *jira.BoardConfiguration, filterJQL string, api jira.Api) app.View {
	col := 0
	statusesColumnsMap := map[string]int{}
	columnStatusesMap := map[int][]string{}
	columns := make([]string, 0, 20)
	for _, column := range boardConfiguration.ColumnConfig.Columns {
		if len(column.Statuses) == 0 {
			continue
		}
		if columnStatusesMap[col] == nil {
			columnStatusesMap[col] = make([]string, 0, 20)
		}
		for _, status := range column.Statuses {
			statusesColumnsMap[status.Id] = col
			columnStatusesMap[col] = append(columnStatusesMap[col], status.Id)
		}
		columns = append(columns, column.Name)
		col++
	}
	bottomBar := ui.CreateBottomLeftBar()
	bottomBar.AddItem(ui.CreateArrowsNavigateItem())
	bottomBar.AddItem(ui.CreateSelectItem())
	bottomBar.AddItem(ui.NewAssigneeFilterBarItem())
	bottomBar.AddItem(ui.NewOpenBarItem())
	bottomBar.AddItem(ui.NewCancelBarItem())
	selectedIssueBottomBar := ui.CreateBottomLeftBar()
	selectedIssueBottomBar.AddItem(ui.CreateMoveArrowsItem())
	selectedIssueBottomBar.AddItem(ui.CreateUnSelectItem())
	selectedIssueBottomBar.AddItem(ui.NewCancelBarItem())
	topBar := ui.CreateIssueTopBar(&jira.Issue{})
	return &boardView{
		api:                    api,
		project:                project,
		boardConfiguration:     boardConfiguration,
		filterJQL:              filterJQL,
		statusesColumnsMap:     statusesColumnsMap,
		columnStatusesMap:      columnStatusesMap,
		columns:                columns,
		columnsX:               map[int]int{},
		issuesRow:              map[string]int{},
		issuesColumn:           map[string]int{},
		issuesSummaries:        map[string]string{},
		cursorX:                0,
		cursorY:                0,
		bottomBar:              bottomBar,
		selectedIssueBottomBar: selectedIssueBottomBar,
		topBar:                 topBar,
		scrollX:                0,
		highlightedIssue:       &jira.Issue{},
		columnSize:             28,
		columnHeaderStyle:      app.DefaultStyle().Background(app.Color("boards.headers.background")).Foreground(app.Color("boards.headers.foreground")),
		issueStyle:             app.DefaultStyle().Foreground(app.Color("boards.column.foreground")),
		highlightIssueStyle:    app.DefaultStyle().Foreground(app.Color("boards.highlight.foreground")).Background(app.Color("boards.highlight.background")),
		selectedIssueStyle:     app.DefaultStyle().Background(app.Color("boards.selection.background")).Foreground(app.Color("boards.selection.foreground")).Bold(true),
		titleStyle:             app.DefaultStyle().Italic(true).Foreground(app.Color("boards.title.foreground")),
	}
}

func (b *boardView) Draw(screen tcell.Screen) {
	if len(b.issues) == 0 {
		b.drawColumnsHeaders(screen)
		b.topBar.Draw(screen)
		return
	}
	for _, issue := range b.issues {
		column := b.statusesColumnsMap[issue.Fields.Status.Id]
		x := b.columnsX[column]
		y := b.issuesRow[issue.Id]
		// do not draw issues at the bottom of top-bar&headers
		if y+topMargin-b.scrollY < topMargin {
			continue
		}
		if b.highlightedIssue.Id == issue.Id {
			var style = &b.highlightIssueStyle
			if b.issueSelected {
				style = &b.selectedIssueStyle
			}
			app.DrawTextLimited(screen, x-b.scrollX, y+topMargin-b.scrollY, x+b.columnSize-b.scrollX, y+1+topMargin, *style, b.issuesSummaries[issue.Id])
			continue
		}
		app.DrawTextLimited(screen, x-b.scrollX, y+topMargin-b.scrollY, x+b.columnSize-b.scrollX, y+1+topMargin, b.issueStyle, b.issuesSummaries[issue.Id])
	}
	if b.highlightedIssue != nil {
		app.DrawText(screen, 0, 1, b.titleStyle, app.WriteIndicator)
		app.DrawText(screen, 2, 1, b.titleStyle, b.issuesSummaries[b.highlightedIssue.Id])
	}
	if !b.issueSelected {
		b.bottomBar.Draw(screen)
	} else {
		b.selectedIssueBottomBar.Draw(screen)
	}
	b.drawColumnsHeaders(screen)
	b.topBar.Draw(screen)
	b.ensureHighlightInViewport()
}

func (b *boardView) Update() {
	b.bottomBar.Update()
	b.selectedIssueBottomBar.Update()
	b.topBar.Update()
}

func (b *boardView) Resize(screenX, screenY int) {
	b.bottomBar.Resize(screenX, screenY)
	b.selectedIssueBottomBar.Resize(screenX, screenY)
	b.topBar.Resize(screenX, screenY)
	b.screenY = screenY
	b.screenX = screenX
	for i := range b.columns {
		if i == 0 {
			b.columnsX[i] = 0
		} else {
			b.columnsX[i] = i*b.columnSize + i
		}
	}
}

func (b *boardView) Init() {
	app.GetApp().Loading(true)
	b.allIssues = make([]jira.Issue, 0, maxIssuesNumber)
	page := int32(0)
	for len(b.allIssues) < maxIssuesNumber {
		iss, total, _, err := b.api.SearchJqlPageable(b.filterJQL, page, issueFetchBatchSize)
		if err != nil {
			app.GetApp().Loading(false)
			app.Error(err.Error())
			return
		}
		b.allIssues = append(b.allIssues, iss...)
		if len(b.allIssues) >= int(total) {
			break
		}
		page++
	}

	// Initialize issues: if a filter is active, reapply it
	if b.assigneeFilter != nil {
		b.applyAssigneeFilter(b.assigneeFilter)
	} else {
		b.issues = make([]jira.Issue, len(b.allIssues))
		copy(b.issues, b.allIssues)
		b.refreshIssuesSummaries()
		b.refreshIssuesRows()
		b.setInitialCursorX()
		b.refreshHighlightedIssue()
	}
	app.GetApp().Loading(false)
	go b.handleActions()
}

func (b *boardView) Destroy() {
	// ...
}

func (b *boardView) Refresh() {
	b.refreshIssuesSummaries()
	b.refreshIssuesRows()
	b.refreshIssueTopBar()
	b.refreshHighlightedIssue()
}

func (b *boardView) SetColumnSize(colSize int) {
	b.columnSize = colSize
	b.Resize(b.scrollX, b.screenY)
	b.Refresh()
}

func (b *boardView) SetGoBackFn(f func()) {
	b.goBackFn = f
}

func (b *boardView) HandleKeyEvent(ev *tcell.EventKey) {
	if app.GetApp().IsLoading() {
		return
	}
	if !b.issueSelected {
		b.bottomBar.HandleKeyEvent(ev)
	} else {
		b.selectedIssueBottomBar.HandleKeyEvent(ev)
	}
	if ev.Key() == tcell.KeyEnter {
		// If not in edit/move mode, open issue detail view
		if !b.issueSelected && b.highlightedIssue != nil && b.highlightedIssue.Id != "" {
			app.GoTo("issue", b.highlightedIssue.Id, b.reopen, b.api)
			return
		}
	}
	if ev.Key() == tcell.KeyRight || ev.Rune() == vimRight {
		b.moveCursorRight(0)
	}
	if ev.Key() == tcell.KeyLeft || ev.Rune() == vimLeft {
		b.moveCursorLeft(0)
	}
	if ev.Key() == tcell.KeyUp || ev.Rune() == vimUp {
		b.cursorY = app.MaxInt(0, b.cursorY-1)
		b.refreshHighlightedIssue()
	}
	if ev.Key() == tcell.KeyDown || ev.Rune() == vimDown {
		// TODO - get number of issues in column
		b.cursorY = app.MinInt(1000, b.cursorY+1)
		b.refreshHighlightedIssue()
	}
}

func (b *boardView) drawColumnsHeaders(screen tcell.Screen) {
	b.tmpX = 0
	for _, column := range b.columns {
		app.DrawText(screen, b.tmpX-b.scrollX, topMargin, b.columnHeaderStyle, centerString(column, b.columnSize))
		b.tmpX += b.columnSize + 1
	}
}

func (b *boardView) moveCursorRight(recursionDepth ...int) {
	maxDepth := 20 // Prevent infinite recursion
	depth := 0
	if len(recursionDepth) > 0 {
		depth = recursionDepth[0]
	}
	if depth > maxDepth {
		debugLog("[DEBUG] moveCursorRight: max recursion depth reached, aborting to prevent freeze")
		return
	}
	if b.cursorX+1 >= len(b.columns) {
		debugLog("[DEBUG] moveCursorRight: already at rightmost column, cannot move further right")
		return
	}
	b.cursorX = app.MinInt(len(b.columns)-1, b.cursorX+1)
	b.cursorY = 0
	if b.issueSelected {
		b.moveIssue(b.highlightedIssue, 1)
		return
	}
	// no issues in a column
	if f := b.refreshHighlightedIssue(); !f {
		b.moveCursorRight(depth + 1)
		return
	}
	b.scrollY = 0
}

func (b *boardView) moveCursorLeft(recursionDepth ...int) {
	maxDepth := 20 // Prevent infinite recursion
	depth := 0
	if len(recursionDepth) > 0 {
		depth = recursionDepth[0]
	}
	if depth > maxDepth {
		debugLog("[DEBUG] moveCursorLeft: max recursion depth reached, aborting to prevent freeze")
		return
	}
	if b.cursorX-1 < 0 {
		debugLog("[DEBUG] moveCursorLeft: already at leftmost column, cannot move further left")
		return
	}
	b.cursorX = app.MaxInt(0, b.cursorX-1)
	b.cursorY = 0
	if b.issueSelected {
		b.moveIssue(b.highlightedIssue, -1)
		return
	}
	// no issues in a column
	if f := b.refreshHighlightedIssue(); !f {
		b.moveCursorLeft(depth + 1)
		return
	}
	b.scrollY = 0
}

func (b *boardView) handleActions() {
	defer app.GetApp().PanicRecover()
	for {
		select {
		case action := <-b.bottomBar.Action:
			switch action {
			case ui.ActionSelect:
				if b.highlightedIssue != nil && b.highlightedIssue.Id != "" {
					app.GoTo("issue", b.highlightedIssue.Id, b.reopen, b.api)
				}
			case ui.ActionSearchByAssignee:
				b.runSelectAssigneeFilter()
				return // Stop this action handler
			case ui.ActionCancel:
				if b.goBackFn != nil {
					b.goBackFn()
				}
			case ui.ActionOpen:
				app.GoTo("issue", b.highlightedIssue.Id, b.reopen, b.api)
			}
		case action := <-b.selectedIssueBottomBar.Action:
			switch action {
			case ui.ActionUnselect:
				b.issueSelected = false
			case ui.ActionCancel:
				b.issueSelected = false
			case ui.ActionOpen:
				app.GoTo("issue", b.highlightedIssue.Id, b.reopen, b.api)
			}
		default: //nolint
		}
	}
}

func (b *boardView) reopen() {
	app.GetApp().SetView(b)
}

func (b *boardView) refreshIssueTopBar() {
	b.topBar.GetItem(0).ChangeText2(b.highlightedIssue.Key)
	b.topBar.GetItem(1).ChangeText2(b.highlightedIssue.Fields.Reporter.DisplayName)
	b.topBar.GetItem(2).ChangeText2(b.highlightedIssue.Fields.Assignee.DisplayName)
	b.topBar.GetItem(3).ChangeText2(b.highlightedIssue.Fields.Type.Name)
	b.topBar.GetItem(4).ChangeText2(b.highlightedIssue.Fields.Status.Name)
	b.topBar.Resize(b.screenX, b.screenY)
}

func (b *boardView) refreshHighlightedIssue() bool {
	for i, issue := range b.issues {
		y := b.issuesRow[issue.Id]
		if b.issuesColumn[issue.Id] == b.cursorX && y-1 == b.cursorY {
			if b.highlightedIssue.Key != issue.Key {
				b.highlightedIssue = &b.issues[i]
				b.refreshIssueTopBar()
				b.ensureHighlightInViewport()
				return true
			}
		}
	}
	return false
}

func (b *boardView) pointCursorTo(issueId string) {
	b.cursorX = b.issuesColumn[issueId]
	b.cursorY = b.issuesRow[issueId] - 1
}

func (b *boardView) refreshIssuesRows() {
	rows := map[int]int{}
	for _, issue := range b.issues {
		column := b.statusesColumnsMap[issue.Fields.Status.Id]
		y := rows[column] + 1
		b.issuesRow[issue.Id] = y
		b.issuesColumn[issue.Id] = column
		rows[column] = y
	}
}

func (b *boardView) refreshIssuesSummaries() {
	for _, issue := range b.issues {
		b.issuesSummaries[issue.Id] = fmt.Sprintf("%s %s", issue.Key, issue.Fields.Summary)
	}
}

func (b *boardView) setInitialCursorX() {
	if len(b.issuesColumn) == 0 {
		return
	}
	b.cursorX = len(b.columnsX)
	for _, v := range b.issuesColumn {
		if v < b.cursorX {
			b.cursorX = v
		}
	}
}

func (b *boardView) moveIssue(issue *jira.Issue, direction int) {
	app.GetApp().Loading(true)
	inc := 1
	if direction < 0 {
		inc = -1
	}
	column := b.statusesColumnsMap[issue.Fields.Status.Id] + inc
	targetColumnStatuses := b.columnStatusesMap[column]
	transitions, err := b.api.FindTransitions(issue.Id)
	if err != nil {
		app.GetApp().Loading(false)
		app.Error(err.Error())
		return
	}
	var targetTransition *jira.IssueTransition
	for i, transition := range transitions {
		for _, targetStatus := range targetColumnStatuses {
			if transition.To.StatusId == targetStatus {
				targetTransition = &transitions[i]
				break
			}
		}
	}
	if targetTransition == nil {
		app.GetApp().Loading(false)
		app.Error(ui.MessageCannotFindStatusForColumn)
		return
	}
	err = b.api.DoTransition(issue.Id, targetTransition)
	if err != nil {
		app.GetApp().Loading(false)
		app.Error(err.Error())
		return
	}
	app.GetApp().Loading(false)
	app.Success(fmt.Sprintf(ui.MessageChangeStatusSuccess, issue.Key, targetTransition.To.Name))
	issue.Fields.Status.Id = targetTransition.To.StatusId
	issue.Fields.Status.Name = targetTransition.To.Name
	b.issueSelected = false
	b.refreshIssuesRows()
	b.pointCursorTo(issue.Id)
	b.refreshHighlightedIssue()
}

func (b *boardView) ensureHighlightInViewport() {
	if b.highlightedIssue == nil {
		return
	}
	if b.cursorX == 0 {
		b.scrollX = 0
	} else if b.scrollX+(b.cursorX*b.columnSize)+b.columnSize > b.screenX { // highlighted issue out of screen
		b.scrollX = app.MaxInt(0, (b.cursorX-2)*b.columnSize)
	}
	if b.scrollY+b.cursorY > b.scrollY { // highlighted issue out of screen
		b.scrollY = app.MaxInt(0, b.cursorY-2)
	}
}

func centerString(str string, width int) string {
	if len(str) > width {
		str = str[:width]
	}
	spaces := int(float64(width-len(str)) / 2)
	return strings.Repeat(" ", spaces) + str + strings.Repeat(" ", width-(spaces+len(str)))
}

func (b *boardView) runSelectAssigneeFilter() {
	debugLog("[DEBUG] runSelectAssigneeFilter called")
	app.GetApp().ClearNow()
	app.GetApp().Loading(true)

	// Get unique assignees from board issues
	assignees := b.getBoardAssignees()
	assignees = append(assignees, jira.User{DisplayName: ui.MessageAll})

	assigneeStrings := users.FormatJiraUsers(assignees)
	fuzzyFind := app.NewFuzzyFind(ui.MessageSelectUser, assigneeStrings)
	app.GetApp().SetView(fuzzyFind)
	app.GetApp().Loading(false)

	// Block here and wait for completion (same pattern as issues view)
	if user := <-fuzzyFind.Complete; true {
		app.GetApp().ClearNow()
		if user.Index >= 0 && len(assignees) > 0 {
			selectedUser := &assignees[user.Index]
			if selectedUser.DisplayName == ui.MessageAll {
				b.clearAssigneeFilter()
			} else {
				b.applyAssigneeFilter(selectedUser)
			}
		}
		// Return to board view and restart action handler
		b.reopen()
		go b.handleActions()
	}
}

func (b *boardView) getBoardAssignees() []jira.User {
	assigneeNames := make(map[string]struct{})
	hasUnassigned := false
	for _, issue := range b.allIssues {
		assignee := issue.Fields.Assignee
		if assignee.DisplayName != "" {
			assigneeNames[assignee.DisplayName] = struct{}{}
		} else {
			hasUnassigned = true
		}
	}
	filtered := make([]jira.User, 0, len(assigneeNames)+1)
	for name := range assigneeNames {
		filtered = append(filtered, jira.User{
			DisplayName: name,
			Name:        name,
			Key:         name,
		})
	}
	if hasUnassigned {
		filtered = append(filtered, jira.User{DisplayName: ui.MessageUnassigned})
	}
	return filtered
}

func (b *boardView) applyAssigneeFilter(user *jira.User) {
	debugLog(fmt.Sprintf("[DEBUG] applyAssigneeFilter: user=%+v", user))
	b.assigneeFilter = user
	b.issues = make([]jira.Issue, 0)

	for _, issue := range b.allIssues {
		assignee := issue.Fields.Assignee
		debugLog(fmt.Sprintf("[DEBUG] Comparing: user.DisplayName=%q assignee.DisplayName=%q user.AccountId=%q assignee.AccountId=%q", user.DisplayName, assignee.DisplayName, user.AccountId, assignee.AccountId))
		if user.DisplayName == ui.MessageUnassigned {
			if assignee.DisplayName == "" {
				b.issues = append(b.issues, issue)
				debugLog(fmt.Sprintf("[DEBUG] Issue %s: matched UNASSIGNED", issue.Key))
			}
		} else {
			if user.AccountId != "" {
				if assignee.AccountId == user.AccountId {
					b.issues = append(b.issues, issue)
					debugLog(fmt.Sprintf("[DEBUG] Issue %s: matched by AccountId", issue.Key))
				}
			} else if assignee.DisplayName == user.DisplayName {
				b.issues = append(b.issues, issue)
				debugLog(fmt.Sprintf("[DEBUG] Issue %s: matched by DisplayName", issue.Key))
			}
		}
	}
	debugLog(fmt.Sprintf("[DEBUG] Filtered issues count: %d", len(b.issues)))
	b.refreshIssuesSummaries()
	b.refreshIssuesRows()
	b.cursorX = 0
	b.cursorY = 0
	b.scrollX = 0
	b.scrollY = 0
	b.setInitialCursorX()
	b.refreshHighlightedIssue()
}


func (b *boardView) clearAssigneeFilter() {
	b.assigneeFilter = nil
	b.issues = make([]jira.Issue, len(b.allIssues))
	copy(b.issues, b.allIssues)

	b.refreshIssuesSummaries()
	b.refreshIssuesRows()
	b.cursorX = 0
	b.cursorY = 0
	b.scrollX = 0
	b.scrollY = 0
	b.setInitialCursorX()
	b.refreshHighlightedIssue()
}
