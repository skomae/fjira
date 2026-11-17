package commands

import (
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/mk-5/fjira/internal/cache"
	"github.com/mk-5/fjira/internal/jira"
	"github.com/spf13/cobra"
)

// The headless write subcommands (comment, transition, assign) change a Jira
// ticket non-interactively and print one JSON document describing what was
// (or, with --dry-run, would be) done. They exist for agents acting on a
// human's approval: --dry-run resolves everything a real run would (issue,
// transition, user) using only GET requests, so the plan can be shown and
// approved before the write is made.
//
// Every document carries:
//   - the cache envelope (v, kind, workspace, fetchedAt);
//   - key;
//   - dryRun: true when nothing was written;
//   - changed: whether the action changes the ticket. On a dry run it says
//     whether a real run would change it, so a no-op is visible up front;
//   - issue (real runs only): the issue re-fetched after the write, which
//     also refreshes the on-disk cache.

var headlessIssueKeyRegexp = regexp.MustCompile("^[A-Za-z0-9]{2,10}-[0-9]+$")

const maxListedCandidates = 10

type writeResult struct {
	cache.Envelope
	Key     string            `json:"key"`
	DryRun  bool              `json:"dryRun"`
	Changed bool              `json:"changed"`
	Issue   *cache.IssueEntry `json:"issue,omitempty"`
}

type commentResult struct {
	writeResult
	Body string `json:"body"`
}

type transitionEntry struct {
	Id   string `json:"id"`
	Name string `json:"name"`
	To   string `json:"to"`
}

type transitionResult struct {
	writeResult
	From       string           `json:"from"`
	Transition *transitionEntry `json:"transition"`
}

type transitionsList struct {
	cache.Envelope
	Key         string            `json:"key"`
	From        string            `json:"from"`
	Transitions []transitionEntry `json:"transitions"`
}

type userEntry struct {
	DisplayName string `json:"displayName"`
	Email       string `json:"email,omitempty"`
	AccountId   string `json:"accountId,omitempty"`
}

type assignResult struct {
	writeResult
	From     *userEntry `json:"from"`
	Assignee userEntry  `json:"assignee"`
}

func GetHeadlessCommentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "comment [issueKey]",
		Short:         "Comment on an issue and print the result as JSON",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true, // main prints the error once, to stderr
		RunE: func(cmd *cobra.Command, args []string) error {
			key, err := validIssueKey(args[0])
			if err != nil {
				return err
			}
			body, err := commentBody(cmd)
			if err != nil {
				return err
			}
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			return runHeadless(cmd, func(api jira.Api, workspace string) (any, error) {
				// fetch first: validates the key (and access) on dry runs too
				if _, err := api.GetIssueDetailed(key); err != nil {
					return nil, err
				}
				res := commentResult{writeResult: newWriteResult("comment", workspace, key, dryRun, true), Body: body}
				if dryRun {
					return res, nil
				}
				if err := api.DoComment(key, body); err != nil {
					return nil, err
				}
				res.Issue, err = refetch(api, key, workspace)
				return res, err
			})
		},
	}
	cmd.Flags().String("body", "", "comment text")
	cmd.Flags().String("body-file", "", "read the comment text from a file ('-' for stdin)")
	cmd.MarkFlagsMutuallyExclusive("body", "body-file")
	cmd.MarkFlagsOneRequired("body", "body-file")
	addDryRunFlag(cmd)
	markJsonRequired(cmd)
	return cmd
}

func GetHeadlessTransitionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "transition [issueKey]",
		Short:         "Transition an issue (or list its transitions) as JSON",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true, // main prints the error once, to stderr
		RunE: func(cmd *cobra.Command, args []string) error {
			key, err := validIssueKey(args[0])
			if err != nil {
				return err
			}
			to, _ := cmd.Flags().GetString("to")
			list, _ := cmd.Flags().GetBool("list")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			if !list && strings.TrimSpace(to) == "" {
				return errors.New("--to must not be empty")
			}
			return runHeadless(cmd, func(api jira.Api, workspace string) (any, error) {
				issue, err := api.GetIssueDetailed(key)
				if err != nil {
					return nil, err
				}
				from := issue.Fields.Status.Name
				transitions, err := api.FindTransitions(key)
				if err != nil {
					return nil, err
				}
				if list {
					doc := transitionsList{Envelope: cache.NewEnvelope("transitions", workspace), Key: key, From: from, Transitions: transitionEntries(transitions)}
					return doc, nil
				}
				res := transitionResult{writeResult: newWriteResult("transition", workspace, key, dryRun, false), From: from}
				// already there: a no-op, not an error
				if strings.EqualFold(strings.TrimSpace(to), from) {
					return res, nil
				}
				if len(transitions) == 0 {
					return nil, fmt.Errorf("no transitions available for %s", key)
				}
				t, err := matchTransition(transitions, to)
				if err != nil {
					return nil, fmt.Errorf("%s: %w", key, err)
				}
				entry := toTransitionEntry(t)
				res.Transition = &entry
				if strings.EqualFold(t.To.Name, from) {
					return res, nil
				}
				res.Changed = true
				if dryRun {
					return res, nil
				}
				if err := api.DoTransition(key, t); err != nil {
					return nil, err
				}
				res.Issue, err = refetch(api, key, workspace)
				return res, err
			})
		},
	}
	cmd.Flags().String("to", "", "target transition name or status name (case-insensitive; a unique prefix is enough)")
	cmd.Flags().Bool("list", false, "list the available transitions instead of transitioning")
	cmd.MarkFlagsMutuallyExclusive("to", "list")
	cmd.MarkFlagsOneRequired("to", "list")
	addDryRunFlag(cmd)
	markJsonRequired(cmd)
	return cmd
}

func GetHeadlessAssignCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "assign [issueKey]",
		Short:         "Assign an issue and print the result as JSON",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true, // main prints the error once, to stderr
		RunE: func(cmd *cobra.Command, args []string) error {
			key, err := validIssueKey(args[0])
			if err != nil {
				return err
			}
			query, _ := cmd.Flags().GetString("to")
			me, _ := cmd.Flags().GetBool("me")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			if !me && strings.TrimSpace(query) == "" {
				return errors.New("--to must not be empty")
			}
			return runHeadless(cmd, func(api jira.Api, workspace string) (any, error) {
				if me {
					query = cmdWorkspaceSettings(cmd).JiraUsername
					if query == "" {
						return nil, errors.New("--me: the workspace has no jiraUsername configured")
					}
				}
				issue, err := api.GetIssueDetailed(key)
				if err != nil {
					return nil, err
				}
				project := key[:strings.LastIndex(key, "-")]
				users, err := api.FindUsersWithQuery(project, query)
				if err != nil {
					return nil, err
				}
				user, err := matchUser(users, query)
				if err != nil {
					return nil, fmt.Errorf("%s: %w", key, err)
				}
				res := assignResult{writeResult: newWriteResult("assign", workspace, key, dryRun, true), Assignee: toUserEntry(user)}
				if a := issue.Fields.Assignee; a.AccountId != "" || a.DisplayName != "" {
					res.From = &userEntry{DisplayName: a.DisplayName, AccountId: a.AccountId}
					if a.AccountId != "" && a.AccountId == user.AccountId {
						res.Changed = false
						return res, nil
					}
				}
				if dryRun {
					return res, nil
				}
				if err := api.DoAssignee(key, user); err != nil {
					return nil, err
				}
				res.Issue, err = refetch(api, key, workspace)
				return res, err
			})
		},
	}
	cmd.Flags().String("to", "", "user to assign: email, display name or account id (resolved among the project's assignable users)")
	cmd.Flags().Bool("me", false, "assign to the workspace's own user (its jiraUsername)")
	cmd.MarkFlagsMutuallyExclusive("to", "me")
	cmd.MarkFlagsOneRequired("to", "me")
	addDryRunFlag(cmd)
	markJsonRequired(cmd)
	return cmd
}

func addDryRunFlag(cmd *cobra.Command) {
	cmd.Flags().Bool("dry-run", false, "resolve and print what would be done, without writing to jira")
}

func newWriteResult(kind string, workspace string, key string, dryRun bool, changed bool) writeResult {
	return writeResult{Envelope: cache.NewEnvelope(kind, workspace), Key: key, DryRun: dryRun, Changed: changed}
}

func validIssueKey(key string) (string, error) {
	if !headlessIssueKeyRegexp.MatchString(key) {
		return "", fmt.Errorf("%w: %q", ErrInvalidIssueKeyFormat, key)
	}
	return strings.ToUpper(key), nil
}

// refetch re-reads the issue after a write. The api is cache-decorated, so
// this also refreshes the on-disk cache entry.
func refetch(api jira.Api, key string, workspace string) (*cache.IssueEntry, error) {
	issue, err := api.GetIssueDetailed(key)
	if err != nil {
		return nil, fmt.Errorf("write succeeded but re-fetching %s failed: %w", key, err)
	}
	entry := cache.IssueEntryFromJira(issue, api.GetApiUrl())
	entry.Envelope = cache.NewEnvelope("issue", workspace)
	return &entry, nil
}

func commentBody(cmd *cobra.Command) (string, error) {
	var body string
	if path, _ := cmd.Flags().GetString("body-file"); cmd.Flags().Changed("body-file") {
		var raw []byte
		var err error
		if path == "-" {
			raw, err = io.ReadAll(cmd.InOrStdin())
		} else {
			raw, err = os.ReadFile(path)
		}
		if err != nil {
			return "", fmt.Errorf("cannot read --body-file: %w", err)
		}
		body = string(raw)
	} else {
		body, _ = cmd.Flags().GetString("body")
	}
	if strings.TrimSpace(body) == "" {
		return "", errors.New("comment body is empty")
	}
	return body, nil
}

// matchTransition resolves query (case-insensitively) against the
// transitions' names and target status names: an exact transition-name
// match first, then an exact target-status match, then a unique prefix of
// either. Anything else is an error listing the options.
func matchTransition(transitions []jira.IssueTransition, query string) (*jira.IssueTransition, error) {
	q := strings.ToLower(strings.TrimSpace(query))
	matchers := []func(t jira.IssueTransition) bool{
		func(t jira.IssueTransition) bool { return strings.ToLower(t.Name) == q },
		func(t jira.IssueTransition) bool { return strings.ToLower(t.To.Name) == q },
		func(t jira.IssueTransition) bool {
			return strings.HasPrefix(strings.ToLower(t.Name), q) || strings.HasPrefix(strings.ToLower(t.To.Name), q)
		},
	}
	for _, matches := range matchers {
		var found []jira.IssueTransition
		for _, t := range transitions {
			if matches(t) {
				found = append(found, t)
			}
		}
		switch {
		case len(found) == 1:
			return &found[0], nil
		case len(found) > 1:
			return nil, fmt.Errorf("transition %q is ambiguous; matches: %s", query, describeTransitions(found))
		}
	}
	return nil, fmt.Errorf("no transition matches %q; available: %s", query, describeTransitions(transitions))
}

func describeTransitions(transitions []jira.IssueTransition) string {
	parts := make([]string, 0, len(transitions))
	for _, t := range transitions {
		parts = append(parts, fmt.Sprintf("%q (id %s) -> %q", t.Name, t.Id, t.To.Name))
	}
	return strings.Join(parts, ", ")
}

func transitionEntries(transitions []jira.IssueTransition) []transitionEntry {
	entries := make([]transitionEntry, 0, len(transitions))
	for _, t := range transitions {
		entries = append(entries, toTransitionEntry(&t))
	}
	return entries
}

func toTransitionEntry(t *jira.IssueTransition) transitionEntry {
	return transitionEntry{Id: t.Id, Name: t.Name, To: t.To.Name}
}

// matchUser picks the assignee among the assignable-user search results: the
// only result, or the one whose email, display name, account id (or, on
// premise, username) equals query case-insensitively. Anything else is an
// error listing up to maxListedCandidates candidates.
func matchUser(users []jira.User, query string) (*jira.User, error) {
	if len(users) == 0 {
		return nil, fmt.Errorf("no assignable user matches %q", query)
	}
	if len(users) == 1 {
		return &users[0], nil
	}
	q := strings.TrimSpace(query)
	var exact []jira.User
	for _, u := range users {
		for _, field := range []string{u.EmailAddress, u.DisplayName, u.AccountId, u.Name} {
			if field != "" && strings.EqualFold(field, q) {
				exact = append(exact, u)
				break
			}
		}
	}
	if len(exact) == 1 {
		return &exact[0], nil
	}
	candidates := users
	if len(exact) > 1 {
		candidates = exact
	}
	return nil, fmt.Errorf("user %q is ambiguous (%d candidates): %s", query, len(candidates), describeUsers(candidates))
}

func describeUsers(users []jira.User) string {
	parts := make([]string, 0, maxListedCandidates)
	for i, u := range users {
		if i == maxListedCandidates {
			parts = append(parts, fmt.Sprintf("... and %d more", len(users)-maxListedCandidates))
			break
		}
		if u.EmailAddress != "" {
			parts = append(parts, fmt.Sprintf("%s <%s>", u.DisplayName, u.EmailAddress))
		} else {
			parts = append(parts, u.DisplayName)
		}
	}
	return strings.Join(parts, ", ")
}

func toUserEntry(u *jira.User) userEntry {
	return userEntry{DisplayName: u.DisplayName, Email: u.EmailAddress, AccountId: u.AccountId}
}
