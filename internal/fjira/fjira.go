package fjira

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/mk-5/fjira/internal/app"
	"github.com/mk-5/fjira/internal/boards"
	"github.com/mk-5/fjira/internal/filters"
	"github.com/mk-5/fjira/internal/issues"
	"github.com/mk-5/fjira/internal/jira"
	"github.com/mk-5/fjira/internal/labels"
	"github.com/mk-5/fjira/internal/projects"
	"github.com/mk-5/fjira/internal/statuses"
	"github.com/mk-5/fjira/internal/ui"
	"github.com/mk-5/fjira/internal/users"
	"github.com/mk-5/fjira/internal/workspaces"
)

const (
	WelcomeMessage = `
    ____    __________  ___ 
   / __/   / /  _/ __ \/   |
  / /___  / // // /_/ / /| |
 / __/ /_/ // // _, _/ ___ |
/_/  \____/___/_/ |_/_/  |_|
                            
The command line tool for Jira.
`
)

var ErrInstallFailed = errors.New("cannot use fjira. Please check error logs in order to install missing packages")

type Fjira struct {
	app       *app.App
	api       jira.Api
	jiraUrl   string
	workspace string
}

// CliArgs TODO - drop it, and use cobra directly
type CliArgs struct {
	ProjectId       string
	BoardId         int
	IssueKey        string
	Workspace       string
	WorkspaceSwitch bool
	WorkspaceEdit   bool
	JqlMode         bool
	FiltersMode     bool
}

var (
	fjiraInstance *Fjira
	fjiraOnce     sync.Once
)

func CreateNewFjira(settings *workspaces.WorkspaceSettings) *Fjira {
	if settings == nil {
		panic("Cannot find appropriate fjira settings!")
	}
	fjiraOnce.Do(func() {
		url := strings.TrimSuffix(settings.JiraRestUrl, "/")
		api, err := jira.NewApi(url, settings.JiraUsername, settings.JiraToken, settings.JiraTokenType)
		if err != nil {
			app.Error(err.Error())
		}
		fjiraInstance = &Fjira{
			app:       app.CreateNewApp(),
			api:       api,
			jiraUrl:   url,
			workspace: settings.Workspace,
		}
	})
	return fjiraInstance
}

func (f *Fjira) Run(args *CliArgs) {
	x := app.ClampInt(f.app.ScreenX/2-18, 0, f.app.ScreenX)
	y := app.ClampInt(f.app.ScreenY/2-4, 0, f.app.ScreenY)
	welcomeText := app.NewText(x, y, app.DefaultStyle(), WelcomeMessage)
	f.app.AddDrawable(welcomeText)
	f.registerGoTos()
	go f.bootstrap(args)
	f.app.Start()
}

func (f *Fjira) Close() {
	f.api.Close()
	if f.app != nil {
		f.app.PanicRecover()
	}
}

func (f *Fjira) registerGoTos() {
	projects.RegisterGoto()
	issues.RegisterGoTo()
	users.RegisterGoTo()
	statuses.RegisterGoTo()
	labels.RegisterGoTo()
	workspaces.RegisterGoTo()
	boards.RegisterGoTo()
	ui.RegisterGoTo()
	filters.RegisterGoTo()
}

func (f *Fjira) bootstrap(args *CliArgs) {
	defer f.app.PanicRecover()
	if args.BoardId != 0 {
		f.openBoardDirect(args.BoardId, args.ProjectId)
		return
	}
	if args.WorkspaceSwitch {
		app.GoTo("workspaces-switch")
		return
	}
	if args.ProjectId != "" {
		// Launched directly with --project, so issues-search is the top-level
		// view — the projects list was never on the stack. Esc backs out of the
		// app rather than dropping into a projects list the user didn't open.
		app.GoTo("issues-search", args.ProjectId, func() {
			f.app.Quit()
		}, f.api)
		return
	}
	if args.IssueKey != "" {
		app.GoTo("issue", args.IssueKey, nil, f.api)
		return
	}
	if args.JqlMode {
		app.GoTo("jql", f.api)
		return
	}
	if args.FiltersMode {
		app.GoTo("filters", f.api)
		return
	}
	time.Sleep(350 * time.Millisecond)
	f.app.RunOnAppRoutine(func() {
		app.GoTo("projects", f.api)
	})
}

// openBoardDirect opens a board view directly from CLI by board ID, resolving
// its associated project automatically when not supplied. Every failure path
// surfaces a flash error so the user isn't left staring at an empty screen.
func (f *Fjira) openBoardDirect(boardId int, projectKey string) {
	boardConfig, err := f.api.GetBoardConfiguration(boardId)
	if err != nil {
		app.Error(fmt.Sprintf("Cannot load board %d: %s", boardId, err.Error()))
		return
	}
	if boardConfig == nil {
		app.Error(fmt.Sprintf("Board %d not found", boardId))
		return
	}
	if projectKey == "" {
		projectKey = boardConfig.Location.Key
	}
	boardItem := &jira.BoardItem{
		Id:   boardConfig.Id,
		Self: boardConfig.Self,
		Name: boardConfig.Name,
		Type: boardConfig.Type,
	}
	if projectKey != "" {
		project, err := f.api.FindProject(projectKey)
		if err != nil {
			app.Error(fmt.Sprintf("Cannot load project %s: %s", projectKey, err.Error()))
			return
		}
		if project == nil {
			app.Error(fmt.Sprintf("Project %s not found", projectKey))
			return
		}
		f.app.RunOnAppRoutine(func() {
			// Pass boardConfig through so goto.go skips its GetBoardConfiguration
			// refetch — we already have it from the bootstrap fetch above.
			app.GoTo("boards", project, boardItem, nil, f.api, boardConfig)
		})
		return
	}
	projects, err := f.api.GetBoardProjects(boardId)
	if err != nil {
		app.Error(fmt.Sprintf("Cannot load projects for board %d: %s", boardId, err.Error()))
		return
	}
	if len(projects) == 0 {
		app.Error(fmt.Sprintf("Board %d has no associated projects", boardId))
		return
	}
	project := &projects[0]
	f.app.RunOnAppRoutine(func() {
		app.GoTo("boards", project, boardItem, nil, f.api)
	})
}
