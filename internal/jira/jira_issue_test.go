package jira

import (
	"net/http"
	"reflect"
	"testing"
)

func Test_httpJiraApi_GetIssueDetailed(t *testing.T) {
	type args struct {
		id string
	}
	tests := []struct {
		name    string
		args    args
		want    *Issue
		wantErr bool
	}{
		{"should get detailed jira issue",
			args{id: "10011"},
			&Issue{
				Key: "JWC-3", Id: "10011",
				Fields: IssueFields{
					Summary:     "Tutorial - create tutorial",
					Description: "Lorem ipsum",
					Project:     Project{Id: "10003", Name: "JIRA WORK CHART", Key: "JWC"},
					Reporter: struct {
						AccountId   string `json:"accountId"`
						DisplayName string `json:"displayName"`
					}(struct {
						AccountId   string
						DisplayName string
					}{"607f55ba074a0b006a6cb482", "Mateusz Kulawik"}),
					Assignee: struct {
						AccountId   string `json:"accountId"`
						DisplayName string `json:"displayName"`
					}(struct {
						AccountId   string
						DisplayName string
					}{"", ""}),
					Type:    IssueType{Name: "Task"},
					Updated: "2022-02-22T00:27:19.792+0100",
					Created: "2021-10-02T22:34:22.521+0200",
					Labels:  []string{"TestLabel"},
					Status:  Status{Id: "10013", Name: "Done"},
					Comment: struct {
						Comments   []Comment `json:"comments"`
						MaxResults int32     `json:"maxResults"`
						Total      int32     `json:"total"`
						StartAt    int32     `json:"startAt"`
					}(struct {
						Comments   []Comment
						MaxResults int32
						Total      int32
						StartAt    int32
					}{
						Comments: []Comment{
							{Body: "Comment 123-ABC", Created: "2022-06-09T22:53:42.057+0200", Author: User{DisplayName: "Mateusz Kulawik"}},
						},
						MaxResults: 1, Total: 1, StartAt: 0},
					),
					// The fixture has "subtasks": [] and "issuelinks": [], which
					// unmarshal to non-nil empty slices (DeepEqual distinguishes
					// those from nil).
					Subtasks:   []IssueRef{},
					IssueLinks: []IssueLink{},
				},
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := NewJiraApiMock(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(200)
				body := `
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
		"labels": ["TestLabel"],
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
}
`
				w.Write([]byte(body)) //nolint:errcheck
			})
			got, err := api.GetIssueDetailed(tt.args.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetIssueDetailed() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetIssueDetailed() got = %v, want %v", got, tt.want)
			}
		})
	}
}

// Proves the standard fields.parent object deserializes into IssueFields.Parent
// (the JSON tags are right) using a body shaped like a real Jira Cloud response.
// A wrong tag would silently leave Parent zero-valued and hide the epic/parent
// row in the UI, so this guards that end-to-end mapping, not just the Go structs.
func Test_httpJiraApi_GetIssueDetailed_parent(t *testing.T) {
	body := `{
        "id": "10011",
        "key": "COINS-692",
        "fields": {
            "summary": "child story",
            "parent": {
                "id": "10455401",
                "key": "COINS-179",
                "fields": {
                    "summary": "Ecomm Activation Recommendations",
                    "issuetype": { "id": "10000", "name": "Epic", "subtask": false }
                }
            }
        }
    }`
	api := NewJiraApiMock(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(body))
	})
	got, err := api.GetIssueDetailed("COINS-692")
	if err != nil {
		t.Fatalf("GetIssueDetailed() error = %v", err)
	}
	if got.Fields.Parent.Key != "COINS-179" {
		t.Errorf("Parent.Key = %q, want COINS-179", got.Fields.Parent.Key)
	}
	if got.Fields.Parent.Fields.Summary != "Ecomm Activation Recommendations" {
		t.Errorf("Parent.Fields.Summary = %q", got.Fields.Parent.Fields.Summary)
	}
	if got.Fields.Parent.Fields.Type.Name != "Epic" {
		t.Errorf("Parent.Fields.Type.Name = %q, want Epic", got.Fields.Parent.Fields.Type.Name)
	}
}

// Proves fields.subtasks and fields.issuelinks deserialize, using bodies shaped
// like real Jira Cloud responses (statusCategory.id is an unquoted number while
// sibling ids are strings — the response below keeps that so a wrong tag would
// surface). Each issuelink carries exactly one of inwardIssue/outwardIssue.
func Test_httpJiraApi_GetIssueDetailed_subtasks_and_links(t *testing.T) {
	body := `{
        "id": "1", "key": "COINS-1",
        "fields": {
            "summary": "parent",
            "subtasks": [
                {
                    "id": "10824552", "key": "AORG-9418",
                    "fields": {
                        "summary": "a done subtask",
                        "status": { "name": "Done", "id": "10076", "statusCategory": { "id": 3, "key": "done", "name": "Done" } },
                        "issuetype": { "id": "10069", "name": "Sub-task", "subtask": true }
                    }
                }
            ],
            "issuelinks": [
                {
                    "id": "2698711",
                    "type": { "name": "Blocks", "inward": "is blocked by", "outward": "blocks" },
                    "inwardIssue": {
                        "key": "AORG-10543",
                        "fields": {
                            "summary": "a blocker",
                            "status": { "name": "In Progress", "id": "3", "statusCategory": { "id": 4, "key": "indeterminate", "name": "In Progress" } }
                        }
                    }
                },
                {
                    "id": "2686485",
                    "type": { "name": "Bonfire Testing", "inward": "Testing discovered", "outward": "Discovered while testing" },
                    "outwardIssue": {
                        "key": "COINS-629",
                        "fields": {
                            "summary": "a done link",
                            "status": { "name": "Closed", "id": "6", "statusCategory": { "id": 3, "key": "done", "name": "Done" } }
                        }
                    }
                }
            ]
        }
    }`
	api := NewJiraApiMock(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(body))
	})
	got, err := api.GetIssueDetailed("COINS-1")
	if err != nil {
		t.Fatalf("GetIssueDetailed() error = %v", err)
	}

	// Subtask populates, and done-detection works via statusCategory.
	if len(got.Fields.Subtasks) != 1 {
		t.Fatalf("Subtasks len = %d, want 1", len(got.Fields.Subtasks))
	}
	st := got.Fields.Subtasks[0]
	if st.Key != "AORG-9418" || st.Fields.Summary != "a done subtask" || !st.Fields.Status.IsDone() {
		t.Errorf("subtask = %+v, IsDone=%v", st, st.Fields.Status.IsDone())
	}

	// Both links populate; inward vs outward resolved by Linked().
	if len(got.Fields.IssueLinks) != 2 {
		t.Fatalf("IssueLinks len = %d, want 2", len(got.Fields.IssueLinks))
	}
	inward := got.Fields.IssueLinks[0]
	if inward.InwardIssue == nil || inward.OutwardIssue != nil {
		t.Errorf("link[0] should be inward-only, got inward=%v outward=%v", inward.InwardIssue, inward.OutwardIssue)
	}
	if l := inward.Linked(); l == nil || l.Key != "AORG-10543" || l.Fields.Status.IsDone() {
		t.Errorf("inward link resolved wrong: %+v", l)
	}
	outward := got.Fields.IssueLinks[1]
	if outward.OutwardIssue == nil || outward.InwardIssue != nil {
		t.Errorf("link[1] should be outward-only")
	}
	if l := outward.Linked(); l == nil || l.Key != "COINS-629" || !l.Fields.Status.IsDone() {
		t.Errorf("outward link resolved wrong: %+v", l)
	}
}
