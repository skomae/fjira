package commands

import (
	"encoding/json"
	"io"

	"github.com/mk-5/fjira/internal/cache"
	"github.com/mk-5/fjira/internal/fjira"
	"github.com/mk-5/fjira/internal/jira"
	"github.com/mk-5/fjira/internal/workspaces"
	"github.com/spf13/cobra"
)

// The headless subcommands print a single JSON document to stdout and quit —
// no TUI. The document shape is identical to the corresponding on-disk cache
// entry (envelope included), and because the fetch goes through the same
// cache-decorated api as the TUI, running them also warms the cache when
// manage.cache.files is enabled. --json is required so that a future default
// (human-readable text) stays open.

func GetHeadlessIssueCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "issue [issueKey]",
		Short:         "Print a Jira issue (with comments) as JSON",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true, // main prints the error once, to stderr
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHeadless(cmd, func(api jira.Api, workspace string) (any, error) {
				issue, err := api.GetIssueDetailed(args[0])
				if err != nil {
					return nil, err
				}
				entry := cache.IssueEntryFromJira(issue, api.GetApiUrl())
				entry.Envelope = cache.NewEnvelope("issue", workspace)
				return entry, nil
			})
		},
	}
	markJsonRequired(cmd)
	return cmd
}

func GetHeadlessSearchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "search [jql]",
		Short:         "Run a JQL search and print the result as JSON",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true, // main prints the error once, to stderr
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHeadless(cmd, func(api jira.Api, workspace string) (any, error) {
				issues, err := api.SearchJql(args[0])
				if err != nil {
					return nil, err
				}
				entry := cache.SearchEntry{Jql: args[0], Issues: cache.SummariesFromJira(issues)}
				entry.Envelope = cache.NewEnvelope("search", workspace)
				return entry, nil
			})
		},
	}
	markJsonRequired(cmd)
	return cmd
}

func GetHeadlessChildrenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "children [issueKey]",
		Short:         "Print an issue's children (epic stories, sub-tasks) as JSON",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true, // main prints the error once, to stderr
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHeadless(cmd, func(api jira.Api, workspace string) (any, error) {
				issues, err := api.SearchJql(cache.ChildrenJql(args[0]))
				if err != nil {
					return nil, err
				}
				entry := cache.ChildrenEntry{Parent: args[0], Issues: cache.SummariesFromJira(issues)}
				entry.Envelope = cache.NewEnvelope("children", workspace)
				return entry, nil
			})
		},
	}
	markJsonRequired(cmd)
	return cmd
}

// runHeadless builds the (cache-decorated) api from the workspace settings
// the root prerun put on the context, runs fetch, and prints its result as
// indented JSON on the command's stdout.
func runHeadless(cmd *cobra.Command, fetch func(api jira.Api, workspace string) (any, error)) error {
	s := cmdWorkspaceSettings(cmd)
	api, err := fjira.CreateApi(s)
	if err != nil {
		return err
	}
	defer api.Close()
	doc, err := fetch(api, s.Workspace)
	if err != nil {
		return err
	}
	return printJson(cmd.OutOrStdout(), doc)
}

func cmdWorkspaceSettings(cmd *cobra.Command) *workspaces.WorkspaceSettings {
	return cmd.Context().Value(CtxWorkspaceSettings).(*workspaces.WorkspaceSettings)
}

func markJsonRequired(cmd *cobra.Command) {
	cmd.Flags().Bool("json", false, "print JSON to stdout (required)")
	_ = cmd.MarkFlagRequired("json")
}

func printJson(w io.Writer, doc any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(doc)
}
