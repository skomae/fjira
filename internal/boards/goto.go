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
		// Optional 5th arg: pre-fetched *jira.BoardConfiguration to skip the refetch.
		var boardConfig *jira.BoardConfiguration
		if len(args) >= 5 {
			if bc, ok := args[4].(*jira.BoardConfiguration); ok {
				boardConfig = bc
			}
		}

		defer app.GetApp().PanicRecover()
		app.GetApp().Loading(true)
		if boardConfig == nil {
			bc, err := api.GetBoardConfiguration(board.Id)
			if err != nil {
				app.GetApp().Loading(false)
				app.Error(err.Error())
				return
			}
			boardConfig = bc
		}
		var sprints []jira.SprintItem
		if boardConfig.Type == "scrum" {
			s, err := api.GetBoardSprints(boardConfig.Id)
			if err != nil {
				app.GetApp().Loading(false)
				app.Error(err.Error())
				return
			}
			sprints = s
		}
		app.GetApp().Loading(false)
		boardView := NewBoardView(project, boardConfig, "", api).(*boardView)
		if sprints != nil {
			boardView.SetSprints(sprints)
		}
		boardView.SetGoBackFn(goBackFn)
		app.GetApp().SetView(boardView)
	})
}
