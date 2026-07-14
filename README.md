# Fjira - Fuzzy finder and TUI application for Jira.

<img src="fjira.png" alt="drawing" width="256"/>

[![Mentioned in Awesome Go](https://awesome.re/badge-flat.svg)](https://github.com/avelino/awesome-go)
![Test](https://github.com/mk-5/fjira/actions/workflows/tests.yml/badge.svg)
[![License: AGPL-3.0-only](https://img.shields.io/badge/License-AGPL--3.0--only-blue.svg)](https://github.com/mk-5/fjira/blob/master/LICENSE)
![GitHub release (latest by date)](https://img.shields.io/github/v/release/mk-5/fjira)
[![Go Report Card](https://goreportcard.com/badge/github.com/mk-5/fjira)](https://goreportcard.com/report/github.com/mk-5/fjira)
[![Go Reference](https://pkg.go.dev/badge/github.com/mk-5/fjira.svg)](https://pkg.go.dev/github.com/mk-5/fjira)
[![codecov](https://codecov.io/gh/mk-5/fjira/branch/master/graph/badge.svg?token=MJBTMYGQQW)](https://codecov.io/gh/mk-5/fjira)

## Introduction

Fjira is a powerful command-line tool designed to simplify your interactions with Jira. Whether you're a developer,
project manager, or just a Jira enthusiast, Fjira streamlines your workflow, making Jira tasks more efficient than ever
before.

![Fjira Demo](demo.gif)

## Key Features

- **Fuzzy-find like interface:** Search for Jira projects and issues with ease.
- **Assignee Control:** Quickly change issue assignees without navigating the Jira interface.
- **Status Updates:** Update Jira issue statuses directly from your terminal.
- **Efficient Comments:** Easily append comments to Jira issues.
- **Multi-Workspace Support:** Manage multiple Jira workspaces effortlessly.
- **Custom Searches:** Use Jira Query Language (JQL) for tailored searches.
- **Direct CLI Access:** Access Jira issues directly from the command line.
- **Cross-Platform Compatibility:** Works seamlessly on macOS, Linux, and Windows.

## Improvements over upstream

This fork builds on [mk-5/fjira](https://github.com/mk-5/fjira) to
close the gap between "glance at Jira from the terminal" and "actually
work in it" — faster ways in, sharper search, richer context on a
ticket, real editing, and the fixes that make Atlassian Cloud usable.
The bullets are grouped by what you're trying to get done; the
technical mechanism is kept as the sub-detail. Several items are good
candidates for upstream PRs; the rest are personal-fit features.

### Jump straight to what you need

Fewer keystrokes between the shell and the ticket you care about.

- **Open a board by id from the shell** — `fjira --board=15391`.
  The fork remembers the last board you viewed and prints its id on
  shutdown, so the next launch is trivially `fjira --board=$(...)`.
- **Faster `--board` startup** — the already-fetched
  `BoardConfiguration` is threaded through to the goto handler
  instead of being refetched, and `GetFilter` is deferred until it's
  actually needed (kanban / no active sprint). That removes an
  unconditional ~8s round trip on scrum boards.
- **Board navigation follows the issues, not the grid** — arrow keys
  skip empty columns and snap to real issue rows. Up/Down moves
  between issues in the current column; Left/Right jumps to the first
  issue of the next non-empty column.
- **Enter drills in, `m` moves** — Enter opens the issue detail on a
  board (matching every other view in fjira); the original
  move-across-columns behavior is on the `m` rune.
- **`Esc` exits cleanly after `--project`** — when launched directly
  into a project, `Esc` now quits instead of dropping you into a
  projects list you never opened. Normal drill-in from the projects
  list is unaffected.

### Find the right issue

The issue browser was reworked to make searching and filtering
precise, and to remember how you like to work.

- **Search matches keys and summaries, nothing else** — fuzzy-find is
  scoped to the issue key and summary, so typing no longer lights up
  characters in the type, status, assignee, or date columns. The
  formatter emits the byte ranges of the matchable columns and a
  range-aware provider maps matched offsets back to the display row,
  so highlighting always lands on the right characters.
- **Search reaches past your active filters** — during a text search,
  any matching issue in the project surfaces, not just ones passing
  the current filters. Filter-aligned matches sort first as a soft
  tiebreak; excluded-status issues still appear but render dimmed and
  sort last.
- **Numbers match issue numbers** — a purely numeric query in a
  selected project searches `key ~ "PROJ-N*"` directly, so `53`
  matches `PROJ-53`, `PROJ-530`, `PROJ-5300`… and never false-matches
  `PROJ-153` or a "53" buried in a description. Alphabetic queries
  can't scatter onto the project prefix either.
- **Exclude statuses you don't care about** — F7 stacks status
  exclusions (multi-select); the top bar shows them as
  `Exclude Status: -Done, -Won't Fix`.
- **Your filters and sort order are remembered per project** — the
  by-status, by-assignee, by-label, and excluded-status filters plus
  the F9 sort mode (status ↔ updated) are saved per connection ×
  project in `fjira.yaml` and restored next time you open that
  project. Only primitive ids/names are stored and the Jira structs
  are rebuilt inline, so restore needs no API round trip. F8 clears
  all four filters at once.
- **More at a glance in the list** — new issue-type (`BUG`, `SPIKE`,
  `EPIC`, …) and friendly "last updated" columns ("2 hours ago",
  "yesterday", "last week"). The assignee column is width-clamped so
  the date lines up.
- **Forgiving Esc in fuzzy-find** — in the issues fuzzy-find, the
  first `Esc` clears the query (fix a typo without losing project
  context); a second `Esc` on an empty query backs out. `Ctrl-C`
  still exits immediately; project / workspace pickers are unchanged.

### Understand an issue at a glance

The issue detail view gained a Details box that answers "what is this,
and what's it connected to?" without opening a browser.

- **Metadata up front** — priority, type, and human-friendly
  created/updated times, plus a relative "Updated" in the top bar.
  Comment headers show a relative date instead of a raw timestamp.
- **Relative time with the exact date beside it** — e.g.
  `2 hours ago (4 Jun 2026 10:35 AM +0200)`, the absolute part
  dimmed so the relative time reads as primary. Jira reports a numeric
  offset rather than a named zone, so the offset is shown verbatim.
- **Parent / epic link** — an issue with a parent leads with
  `Epic <summary> (KEY)` or `Parent <title> (KEY)`. Uses the standard
  `fields.parent` object, which covers both cases across modern Cloud
  and Server — no per-instance Epic-Link customfield is hardcoded.
- **A "Related" column of connected tickets** — sub-tasks and linked
  issues in a right-hand column, each with a leading ✓ when its status
  is in the `done` category (`statusCategory.key == "done"`, robust
  across workflows). For an **epic**, its child stories aren't in the
  epic's own payload, so they're fetched with a `parent = "<KEY>"`
  search issued *after* the view has rendered — the page never blocks
  on it, and results fill in when they land.

### Do the work without leaving the terminal

- **Edit the description in-app** — `d` opens a text-writer modal
  pre-populated with the current description; F2 saves via a
  `DoUpdateDescription` call that PUTs to `/rest/api/2/issue/<id>`,
  `Esc` cancels.
- **A real in-place text editor** — arrow-key navigation, Home/End,
  PgUp/PgDn, Shift for larger jumps, Delete/Backspace at the cursor,
  and mid-text insertion, for both descriptions and comments. The
  cursor glyph is rune-indexed, so multi-byte characters earlier on a
  line no longer shift what's drawn under the cursor.
- **Hand off to `$EDITOR`** — `Ctrl-G` opens the current text in your
  `$EDITOR` (or `$VISUAL`) for anything longer than a quick edit and
  pulls the saved result back in; a non-zero editor exit (e.g. vim
  `:cq`) leaves the text untouched.
- **Filter a board by assignee** — F2 fuzzy-finds over the assignees
  already loaded on the board; "All" clears the filter.
- **Create an issue in context** — F6 from the board, issues list, or
  issue detail opens Jira's create-issue modal in your browser with
  the project (and board, where applicable) pre-populated.
- **Page through long content** — PgUp / PgDn on the issue detail
  complements the existing one-line Up/Down/Tab/Backtab scrolling.

### Works with Atlassian Cloud

Upstream targets Jira Server; these keep the fork usable against
`*.atlassian.net`. On-prem Server behaves identically.

- **Bounded default query** — Cloud's newer `/rest/api/3/search/jql`
  endpoint rejects unbounded queries. When no
  project/status/user/label/query is set, fjira falls back to
  `created >= -30d ORDER BY status` instead of emitting an empty
  restriction set that returns `400`.
- **Expired-token hint** — Cloud returns `200` + an empty project list
  for an invalid/expired API token (not `401`). When the projects list
  is empty on an `*.atlassian.net` host, the flash hint points to
  `https://id.atlassian.com/manage-profile/security/api-tokens`.

### Reliability & correctness fixes

- **Board stability** — crash when paging past the last column; first
  column not scrolling back into view after navigating right then
  left; F2 assignee filter silently cleared on `Init` re-entry.
- **Performance** — eliminated a 100% CPU spin from a busy-wait
  `default:` clause in `handleActions`; fixed a concurrent-map-write
  race in `app.Color()` (now guarded by `sync.Once`).
- **Correct save hint** — the editor save key was rebound from F1 to
  F2 (F1 is easy to mistouch); the comment/description/JQL hints now
  say F2 to match.

## Installation

### Pre-built macOS binary (Apple Silicon)

Grab `fjira` from the
[latest release](https://github.com/skomae/fjira/releases/latest)
and drop it on your `PATH`:

```shell
curl -L -o /usr/local/bin/fjira \
  https://github.com/skomae/fjira/releases/latest/download/fjira
chmod +x /usr/local/bin/fjira
```

The first time you run it macOS Gatekeeper will block the binary
because it's not notarized. Right-click the file in Finder → Open
once to whitelist it, or run:

```shell
xattr -d com.apple.quarantine /usr/local/bin/fjira
```

### Build from source

Any platform with Go 1.25+ installed:

```shell
git clone https://github.com/skomae/fjira.git
cd fjira
make
./out/bin/fjira
```

## Usage

```text
Usage:
  fjira [flags]
  fjira [command]

Available Commands:
  [issueKey]  Open a Jira issue directly from the CLI
  completion  Generate the autocompletion script for the specified shell
  filters     Search using Jira filters
  help        Help about any command
  jql         Search using custom JQL queries
  version     Print the version number of fjira
  workspace   Switch to a different workspace

Flags:
      --board int        Open a board directly from CLI (by board id)
  -h, --help             help for fjira
  -p, --project string   Open a project directly from CLI

Additional help topics:
  fjira            Open a fuzzy finder for projects as a default action

Use "fjira [command] --help" for more information about a command.
```

## Getting Started

Using the Fjira CLI is straightforward. Simply run fjira in your terminal.

```shell
fjira
```

## Workspaces

The first time you run Fjira, it will prompt you for your Jira API URL and token.

![Fjira First Run](demo_first_run.gif)

Fjira workspaces store Jira configuration data in a simple YAML file located at `~/.fjira`. You can switch between
multiple workspaces using the `fjira workspace` command.

```shell
fjira workspace
```

To create a new workspace, use the following command:

```shell
fjira workspace --new abc
```

You can edit an existing workspace using the `--edit` flag:

```shell
fjira workspace --edit abc
```

### Jira Token Type

Fjira supports both Jira Server and Jira Cloud, which use different token types for authorization. The tool will prompt
you to select the appropriate token type during workspace configuration.

```shell
? Jira Token Type:

1. api token
2. personal token

Enter a number (Default is 1):
```

### YAML configuration

If you prefer a manual approach, you have the option to add workspace configurations by creating a `fjira.yaml` file in the `~/.fjira/` directory.
For your convenience, an example configuration file is here: [fjira.yml](assets/fjira.yaml)

## Projects search

The default view when you run `fjira` is the project search screen.

```shell
fjira
```

## Opening a Specific Project

You can open a project directly from the CLI:

```shell
fjira --project=PROJ
```

This will skip the project search screen and take you directly to the issues search screen.

## Opening an Issue Directly

To open an issue directly from the CLI:

```shell
fjira PROJ-123
```

Fjira will skip all intermediate screens and take you directly to the issue view.

![Fjira Issue View](demo_issue.png)

## Boards View

Fjira also offers a board-like view. After opening a project, press F4 to access this view.

![Fjira Board View](demo_board_view.png)

## Custom JQL Queries

You can create and execute custom JQL queries with Fjira:

```shell
fjira jql
```

![Fjira Custom JQL](demo_custom_jql.png)

## My Jira Filters

You can search using your stored (favourites) Jira Filters:

```shell
fjira filters
```

![Fjira Filters](demo_filters.png)

## Custom Color Scheme

Tailor the fjira color scheme to match your preferences by creating a custom `~/.fjira/colors.yml` file. This file
allows you to personalize the colors according to your unique style.
Refer to the example file, located here: [colors.yml](assets/colors.yml)

## Roadmap (TODO)

- Expand Documentation
- Create&Delete Jira Filters
- Introduce More Jira Features

## Motivation

Fjira was designed for personal convenience, born out of a desire for efficiency and a love for terminal tools.
Often, we find ourselves in "I just need to transition issue 123 to the next status." While opening Jira, locating the
ticket on the board, and navigating the Jira issue modal are all perfectly fine, they do consume a fair amount of time.

Fjira empowers you to execute such tasks directly from the terminal, where you're likely already working! 😄

If Fjira enhances your Jira experience as it did mine, please consider giving it a star on GitHub. 🌟 It will power-up me
for a future work.

Feel free to contribute to this project and help shape its future! Your feedback and contributions are highly
appreciated.

## License

This project is licensed under the GNU Affero General Public License v3.0 only (AGPL-3.0-only). See the [LICENSE](LICENSE) file for details.
