package issues

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/mk-5/fjira/internal/app"
	"github.com/mk-5/fjira/internal/jira"
	assert2 "github.com/stretchr/testify/assert"
)

const jiraIssueJson = `
{
    "expand": "renderedFields,names,schema,operations,editmeta,changelog,versionedRepresentations,customfield_10010.requestTypePractice",
    "id": "10011",
    "self": "https://test/rest/api/2/issue/10011",
    "key": "JWC-3",
    "fields": {
        "issuetype": {
            "id": "10013",
            "description": "A small, distinct piece of work.",
            "name": "Task",
            "subtask": false
        },
        "timespent": 14400,
        "project": {
            "id": "10003",
            "key": "JWC",
            "name": "JIRA WORK CHART",
            "projectTypeKey": "software"
        },
        "fixVersions": [],
        "aggregatetimespent": 14400,
        "resolutiondate": "2022-02-22T00:27:11.861+0100",
        "workratio": -1,
        "issuerestriction": {
            "issuerestrictions": {},
            "shouldDisplay": true
        },
        "labels": ["TestLabel"],
        "lastViewed": "2022-02-22T00:27:17.356+0100",
        "created": "2021-10-02T22:34:22.521+0200",
        "aggregatetimeoriginalestimate": null,
        "timeestimate": 0,
        "versions": [],
        "issuelinks": [],
        "assignee": null,
        "updated": "2022-02-22T00:27:19.792+0100",
        "status": {
            "description": "",
            "name": "Done",
            "id": "10013"
        },
        "description": "Lorem ipsum",
        "summary": "Tutorial - create tutorial",
        "creator": {
            "accountId": "607f55ba074a0b006a6cb482",
            "emailAddress": "test@test.dev",
            "displayName": "Mateusz Kulawik",
            "active": true,
            "timeZone": "Europe/Warsaw",
            "accountType": "atlassian"
        },
        "subtasks": [],
        "reporter": {
			"accountId": "607f55ba074a0b006a6cb482",
            "emailAddress": "test@test.dev",
            "displayName": "Mateusz Kulawik",
            "active": true,
            "timeZone": "Europe/Warsaw",
            "accountType": "atlassian"
        },
 		"comment": {
            "comments": [
                {
                    "author": {
                        "displayName": "Mateusz Kulawik"
                    },
                    "body": "Comment 123-ABC",
                    "created": "2022-06-09T22:53:42.057+0200",
                    "updated": "2022-06-09T22:53:42.057+0200"
                }
            ],
            "maxResults": 1,
            "total": 1,
            "startAt": 0
        }
    }
}`

func Test_shouldDisplayIssueView(t *testing.T) {
	screen := tcell.NewSimulationScreen("utf-8")
	_ = screen.Init() //nolint:errcheck
	defer screen.Fini()
	app.InitTestApp(screen)
	// Tall screen so every section (summary, labels, details, description and
	// the comment body) is on-screen at scrollY=0; the Details box plus the
	// scroll buffer push content past a default 25-row viewport otherwise.
	// InitTestApp re-runs screen.Init() (resetting to 80x25), so size after it
	// and mirror the dimensions onto the app that the view's Resize reads.
	screen.SetSize(120, 60)
	app.GetApp().ScreenX = 120
	app.GetApp().ScreenY = 60
	RegisterGoTo()
	api := jira.NewJiraApiMock(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.String(), "issue") {
			w.WriteHeader(200)
			_, _ = w.Write([]byte(jiraIssueJson)) //nolint:errcheck
			return
		}
		if strings.Contains(r.URL.String(), "project") {
			w.WriteHeader(200)
			_, _ = w.Write([]byte("[]")) //nolint:errcheck
		}
	})

	assert := assert2.New(t)
	tests := []struct {
		name     string
		screen   tcell.Screen
		testFunc func()
	}{
		{"should crate valid issue view", screen, func() {
			// when
			app.GoTo("issue", "ABC-123", nil, api)
			view, ok := app.GetApp().CurrentView().(*issueView)

			// then
			assert2.True(t, ok)

			// when
			view.Draw(screen)
			var buffer bytes.Buffer
			contents, x, y := screen.GetContents()
			screen.Show()
			for i := 0; i < x*y; i++ {
				if len(contents[i].Bytes) != 0 {
					buffer.Write(contents[i].Bytes)
				}
			}
			result := buffer.String()

			// then
			assert.True(ok, "Invalid view has been set")
			assert.Equal("JWC-3", view.issue.Key, "Invalid issue key")
			assert.Equal("Lorem ipsum", view.issue.Fields.Description, "Invalid issue description")
			assert.Contains(result, "JWC-3", "should contain issue number")
			assert.Contains(result, "Lorem ipsum", "should contain ticket description")
			assert.Contains(result, "Mateusz Kulawik", "should contain comment")
			assert.Contains(result, "Comment 123-ABC", "should contain comment author")
			assert.Contains(result, "TestLabel", "should contain labels")

			// and then
			view.Destroy()
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.testFunc()
		})
	}
}

func Test_issueView_ActionBar(t *testing.T) {
	app.InitTestApp(nil)
	app.RegisterGoto("issues-search", func(args ...interface{}) {
	})
	app.RegisterGoto("status-change", func(args ...interface{}) {
	})
	app.RegisterGoto("users-assign", func(args ...interface{}) {
	})
	app.RegisterGoto("labels-add", func(args ...interface{}) {
	})
	app.RegisterGoto("text-writer", func(args ...interface{}) {
	})

	type args struct {
		key           tcell.Key
		char          rune
		viewPredicate func() bool
	}
	tests := []struct {
		name string
		args args
	}{
		{"should handle exit action", args{key: tcell.KeyEscape, viewPredicate: func() bool {
			return app.CurrentScreenName() == "issues-search"
		}}},
		{"should handle status change action", args{char: 's', viewPredicate: func() bool {
			return app.CurrentScreenName() == "status-change"
		}}},
		{"should handle assign user action", args{char: 'a', viewPredicate: func() bool {
			return app.CurrentScreenName() == "users-assign"
		}}},
		{"should handle comment action", args{char: 'c', viewPredicate: func() bool {
			return app.CurrentScreenName() == "text-writer"
		}}},
		{"should handle label action", args{char: 'l', viewPredicate: func() bool {
			return app.CurrentScreenName() == "labels-add"
		}}},
		{"should handle open action", args{char: 'o', viewPredicate: func() bool {
			return true
		}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			api := jira.NewJiraApiMock(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(200)
			})
			issue := &jira.Issue{Id: "1", Key: "ABC-1"}
			view := NewIssueView(issue, func() {
				app.GoTo("issues-search", "ABC", nil, jira.NewJiraApiMock(nil))
			}, api).(*issueView)
			done := make(chan struct{})
			started := make(chan struct{})
			go func() {
				started <- struct{}{}
				view.handleIssueAction()
				done <- struct{}{}
			}()
			<-started
			<-time.NewTimer(100 * time.Millisecond).C

			// when
			view.HandleKeyEvent(tcell.NewEventKey(tt.args.key, tt.args.char, tcell.ModNone))
			<-done
			result := tt.args.viewPredicate()

			// then
			assert2.True(t, result)
		})
	}
}

func Test_issueView_doComment(t *testing.T) {
	screen := tcell.NewSimulationScreen("utf-8")
	_ = screen.Init() //nolint:errcheck
	defer screen.Fini()
	app.InitTestApp(screen)

	tests := []struct {
		name string
	}{
		{"should run doComment api"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			done := make(chan bool)
			api := jira.NewJiraApiMock(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(200)
				_, _ = w.Write([]byte(``)) //nolint:errcheck
				done <- true
			})
			view := NewIssueView(&jira.Issue{Key: "test"}, nil, api).(*issueView)

			// when
			view.Init()
			go view.doComment(view.issue, "abcde")

			// then
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Fail()
			}
		})
	}
}

func Test_fjiraIssueView_HandleKeyEvent(t *testing.T) {
	screen := tcell.NewSimulationScreen("utf-8")
	_ = screen.Init() //nolint:errcheck
	defer screen.Fini()
	app.InitTestApp(screen)

	tests := []struct {
		name string
	}{
		{"should process scrollUp&scrollDown&pageUpDown"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// given
			// No fuzzyFind: scrolling is the no-modal behaviour. When the jump
			// modal is open it owns the keyboard and these keys drive its
			// selection instead (covered by Test_issueView_jumpModalOwnsKeyboard).
			view := NewIssueView(&jira.Issue{Key: "test"}, nil, jira.NewJiraApiMock(nil)).(*issueView)
			view.scrollY = 0
			view.maxScrollY = 100

			// when
			view.HandleKeyEvent(tcell.NewEventKey(tcell.KeyDown, 'k', tcell.ModNone))

			// then
			assert2.Equal(t, 1, view.scrollY)

			// and when
			view.HandleKeyEvent(tcell.NewEventKey(tcell.KeyUp, 'k', tcell.ModNone))

			// then
			assert2.Equal(t, 0, view.scrollY)

			// when
			view.HandleKeyEvent(tcell.NewEventKey(tcell.KeyBacktab, 'k', tcell.ModNone))

			// then
			assert2.Equal(t, 1, view.scrollY)

			// and when
			view.HandleKeyEvent(tcell.NewEventKey(tcell.KeyTab, 'k', tcell.ModNone))

			// then
			assert2.Equal(t, 0, view.scrollY)

			// test page navigation
			view.screenY = 40 // set screen height for page size calculation
			expectedPageSize := view.calculatePageSize()

			// when page down
			view.HandleKeyEvent(tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModNone))

			// then
			assert2.Equal(t, expectedPageSize, view.scrollY)

			// and when page up
			view.HandleKeyEvent(tcell.NewEventKey(tcell.KeyPgUp, 0, tcell.ModNone))

			// then
			assert2.Equal(t, 0, view.scrollY)

			// test page down multiple times
			view.HandleKeyEvent(tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModNone))
			view.HandleKeyEvent(tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModNone))

			// then should not exceed maxScrollY
			assert2.Equal(t, expectedPageSize*2, view.scrollY)
		})
	}
}

// While the jump modal is open it owns the keyboard: scroll keys drive the
// modal, not the issue view underneath, so scrollY must not move.
func Test_issueView_jumpModalOwnsKeyboard(t *testing.T) {
	screen := tcell.NewSimulationScreen("utf-8")
	_ = screen.Init() //nolint:errcheck
	defer screen.Fini()
	app.InitTestApp(screen)

	view := NewIssueView(&jira.Issue{Key: "test"}, nil, jira.NewJiraApiMock(nil)).(*issueView)
	view.fuzzyFind = app.NewFuzzyFind("jump", []string{"A-1 one", "A-2 two"})
	view.scrollY = 0
	view.maxScrollY = 100

	view.HandleKeyEvent(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	view.HandleKeyEvent(tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModNone))

	assert2.Equal(t, 0, view.scrollY, "scroll keys must not move the view while the modal is open")
}

// runJumpToRelated with no related tickets is a no-op that re-arms the action
// handler without opening a modal.
func Test_issueView_runJumpToRelated_noRelated(t *testing.T) {
	screen := tcell.NewSimulationScreen("utf-8")
	_ = screen.Init() //nolint:errcheck
	defer screen.Fini()
	app.InitTestApp(screen)

	view := NewIssueView(&jira.Issue{Key: "test"}, nil, jira.NewJiraApiMock(nil)).(*issueView)
	view.relatedKeys = nil

	view.runJumpToRelated()

	assert2.Nil(t, view.fuzzyFind, "no modal should open when there are no related tickets")
}

// Selecting a related ticket in the jump modal navigates to it and closes the
// modal. The Complete channel is served on a goroutine so runJumpToRelated's
// blocking receive returns.
func Test_issueView_runJumpToRelated_opensSelected(t *testing.T) {
	screen := tcell.NewSimulationScreen("utf-8")
	_ = screen.Init() //nolint:errcheck
	defer screen.Fini()
	app.InitTestApp(screen)

	var gotKey string
	app.RegisterGoto("issue", func(args ...interface{}) {
		gotKey = args[0].(string)
	})

	view := NewIssueView(&jira.Issue{Key: "test"}, nil, jira.NewJiraApiMock(nil)).(*issueView)
	view.relatedRows = []string{"  A-1 one", "  A-2 two"}
	view.relatedKeys = []string{"A-1", "A-2"}

	done := make(chan struct{})
	go func() {
		view.runJumpToRelated()
		close(done)
	}()

	// Wait for the modal to be created, then complete it selecting index 1.
	assert2.Eventually(t, func() bool { return view.fuzzyFind != nil }, time.Second, 5*time.Millisecond)
	view.fuzzyFind.Complete <- app.FuzzyFindResult{Index: 1, Match: "  A-2 two"}
	<-done

	assert2.Equal(t, "A-2", gotKey, "should navigate to the selected related issue")
	assert2.Nil(t, view.fuzzyFind, "modal should be cleared after selection")
}
