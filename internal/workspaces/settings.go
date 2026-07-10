package workspaces

import (
	"errors"
	"fmt"
	"os"

	"github.com/mk-5/fjira/internal/jira"
	os2 "github.com/mk-5/fjira/internal/os"
	"gopkg.in/yaml.v3"
)

type Settings struct {
	Current    string `json:"current" yaml:"current"`
	Workspaces map[string]WorkspaceSettings
	// IssueFilters remembers the issue-navigator filters (by status, assignee,
	// label, excluded statuses) the user last viewed, keyed per connection and
	// project so they're restored on the next launch. Key: "<workspace>/<projectId>".
	IssueFilters map[string]ProjectIssueFilters `json:"issueFilters,omitempty" yaml:"issueFilters,omitempty"`
}

// ProjectIssueFilters is the persisted, network-free snapshot of the issue
// navigator's filter state. Only primitive fields are stored (ids + human
// names); the jira structs are reconstructed inline on restore, so no API
// round-trip is needed to repopulate the filters or their top-bar labels.
type ProjectIssueFilters struct {
	StatusId          string   `json:"statusId,omitempty" yaml:"statusId,omitempty"`
	StatusName        string   `json:"statusName,omitempty" yaml:"statusName,omitempty"`
	AssigneeAccountId string   `json:"assigneeAccountId,omitempty" yaml:"assigneeAccountId,omitempty"`
	AssigneeName      string   `json:"assigneeName,omitempty" yaml:"assigneeName,omitempty"`
	AssigneeDisplay   string   `json:"assigneeDisplay,omitempty" yaml:"assigneeDisplay,omitempty"`
	Label             string   `json:"label,omitempty" yaml:"label,omitempty"`
	ExcludedStatusIds []string `json:"excludedStatusIds,omitempty" yaml:"excludedStatusIds,omitempty"`
	// ExcludedStatusNames is index-aligned with ExcludedStatusIds.
	ExcludedStatusNames []string `json:"excludedStatusNames,omitempty" yaml:"excludedStatusNames,omitempty"`
	// SortByUpdated is the F9 sort toggle: true = ORDER BY updated, false =
	// ORDER BY status (the default). Absent in older config unmarshals to false.
	SortByUpdated bool `json:"sortByUpdated,omitempty" yaml:"sortByUpdated,omitempty"`
}

type WorkspaceSettings struct {
	JiraRestUrl   string             `json:"jiraRestUrl" yaml:"jiraRestUrl"`
	JiraToken     string             `json:"jiraToken" yaml:"jiraToken"`
	JiraUsername  string             `json:"jiraUsername" yaml:"jiraUsername"`
	JiraTokenType jira.JiraTokenType `json:"jiraTokenType" yaml:"jiraTokenType"`
	Workspace     string             `json:"-" yaml:"-"`
}

type SettingsStorage interface { //nolint
	Write(workspace string, settings *WorkspaceSettings) error
	Read(workspace string) (*WorkspaceSettings, error)
	ReadAllWorkspaces() ([]string, error)
	ReadCurrentWorkspace() (string, error)
	SetCurrentWorkspace(workspace string) error
	ConfigDir() (string, error)
}

var (
	ErrWorkspaceNotFound = errors.New("workspace doesn't exist")
)

const (
	EmptyWorkspace       = ""
	DefaultWorkspaceName = "default"
	SettingsFilename     = "fjira.yaml"
)

type userHomeSettingsStorage struct{}

func NewUserHomeSettingsStorage() SettingsStorage {
	return &userHomeSettingsStorage{}
}

func (s *userHomeSettingsStorage) Read(workspace string) (*WorkspaceSettings, error) {
	settings, err := s.createOrGetSettings()
	if err != nil {
		return nil, err
	}
	if w, ok := settings.Workspaces[workspace]; ok {
		w.Workspace = workspace
		return &w, nil
	}
	return nil, ErrWorkspaceNotFound
}

func (s *userHomeSettingsStorage) Write(workspace string, workspaceSettings *WorkspaceSettings) error {
	settings, err := s.createOrGetSettings()
	if err != nil {
		return err
	}
	settings.Workspaces[workspace] = *workspaceSettings
	err = s.writeSettings(settings)
	return err
}

func (s *userHomeSettingsStorage) writeSettings(settings *Settings) error {
	settingsFilePath, err := s.settingsFilePath()
	if err != nil {
		return err
	}
	settingsYml, err := yaml.Marshal(settings)
	if err != nil {
		return err
	}
	err = os.WriteFile(settingsFilePath, settingsYml, 0644)
	return err
}

func (s *userHomeSettingsStorage) createOrGetSettings() (*Settings, error) {
	settingsFilePath, err := s.settingsFilePath()
	if err != nil {
		return nil, err
	}
	var settings Settings
	settingsBytes, err := os.ReadFile(settingsFilePath)
	if errors.Is(err, os.ErrNotExist) {
		settings = Settings{
			Current:    DefaultWorkspaceName,
			Workspaces: map[string]WorkspaceSettings{},
		}
	} else if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(settingsBytes, &settings)
	if err != nil {
		return nil, err
	}
	// temporary for migration, "" was a default workspace before. Should be removed after some time
	for k := range settings.Workspaces {
		if k == "" {
			if settings.Current == "" {
				settings.Current = DefaultWorkspaceName
			}
			settings.Workspaces[DefaultWorkspaceName] = settings.Workspaces[k]
			delete(settings.Workspaces, k)
			_ = s.writeSettings(&settings)
			break
		}
	}
	return &settings, nil
}

// WriteIssueFilters persists the issue-navigator filters under the given key
// ("<workspace>/<projectId>"). An empty ProjectIssueFilters is stored as an
// explicit "no filters" entry so restore can distinguish it from a project the
// user has never filtered.
func (s *userHomeSettingsStorage) WriteIssueFilters(key string, filters ProjectIssueFilters) error {
	settings, err := s.createOrGetSettings()
	if err != nil {
		return err
	}
	if settings.IssueFilters == nil {
		settings.IssueFilters = map[string]ProjectIssueFilters{}
	}
	settings.IssueFilters[key] = filters
	return s.writeSettings(settings)
}

// ReadIssueFilters returns the saved filters for the given key and whether an
// entry existed. found=false means the project was never filtered, so callers
// should treat it as "clear all filters".
func (s *userHomeSettingsStorage) ReadIssueFilters(key string) (ProjectIssueFilters, bool, error) {
	settings, err := s.createOrGetSettings()
	if err != nil {
		return ProjectIssueFilters{}, false, err
	}
	f, ok := settings.IssueFilters[key]
	return f, ok, nil
}

func (s *userHomeSettingsStorage) ReadCurrentWorkspace() (string, error) {
	settings, err := s.createOrGetSettings()
	if err != nil {
		return "", err
	}
	return settings.Current, nil
}

func (s *userHomeSettingsStorage) SetCurrentWorkspace(workspace string) error {
	settings, err := s.createOrGetSettings()
	if err != nil {
		return err
	}
	if _, ok := settings.Workspaces[workspace]; ok {
		settings.Current = workspace
		return s.writeSettings(settings)
	}
	return ErrWorkspaceNotFound
}

func (s *userHomeSettingsStorage) ReadAllWorkspaces() ([]string, error) {
	settings, err := s.createOrGetSettings()
	if err != nil {
		return nil, err
	}
	w := make([]string, 0, len(settings.Workspaces))
	for k := range settings.Workspaces {
		w = append(w, k)
	}
	return w, nil
}

func (s *userHomeSettingsStorage) ConfigDir() (string, error) {
	configDir := os2.MustGetFjiraHomeDir()
	if _, err := os.Stat(configDir); errors.Is(err, os.ErrNotExist) {
		err := os.Mkdir(configDir, os.ModePerm)
		if err != nil {
			return "", err
		}
	}
	return configDir, nil
}

func (s *userHomeSettingsStorage) settingsFilePath() (string, error) {
	configDir, err := s.ConfigDir()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%s", configDir, SettingsFilename), err
}
