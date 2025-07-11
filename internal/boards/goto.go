package boards

import (
	"github.com/mk-5/fjira/internal/app"
	"github.com/mk-5/fjira/internal/jira"
)

func RegisterGoTo() {
	app.RegisterGoto("boards", func(args ...interface{}) {
		project := args[0].(*jira.Project)
		board := args[1].(*jira.BoardItem)
		var goBackFn func()
		if fn, ok := args[2].(func()); ok {
			goBackFn = fn
		}
		api := args[3].(jira.Api)

		defer app.GetApp().PanicRecover()
		app.GetApp().Loading(true)
		boardConfig, err := api.GetBoardConfiguration(board.Id)
		if err != nil {
			app.GetApp().Loading(false)
			app.Error(err.Error())
			return
		}
		filter, err := api.GetFilter(boardConfig.Filter.Id)
		if err != nil {
			app.GetApp().Loading(false)
			app.Error(err.Error())
			return
		}

		// The board view now uses GetBoardIssues which automatically handles sprint filtering
		// so we don't need to manually combine the filter JQL with SubQuery
		finalJQL := filter.JQL

		app.GetApp().Loading(false)
		boardView := NewBoardView(project, boardConfig, finalJQL, api).(*boardView)
		boardView.SetGoBackFn(goBackFn)
		app.GetApp().SetView(boardView)
	})
}
