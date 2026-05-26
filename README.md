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

This fork builds on [mk-5/fjira](https://github.com/mk-5/fjira) with
a focus on **board navigation, in-app editing, and Atlassian Cloud
compatibility**. Each item below is a single commit (or small group)
on top of upstream `master`. Several are good candidates for
upstream PRs; the rest are personal-fit features.

### Board view

- **`--board=<id>` CLI arg** — jump straight to a board view from
  the shell. `fjira --board=15391`. The fork remembers the last
  board you viewed and prints its id on shutdown so the next launch
  is trivially `fjira --board=$(...)`.
- **F2 assignee filter** — press F2 on a board view to filter
  visible issues by assignee (fuzzy-find over assignees already
  loaded on the board). Selecting "All" clears the filter.
- **Enter opens issue detail; `m` enters move mode** — Enter is the
  universal "drill in" everywhere else in fjira, so the board view
  now matches. The original "select an issue and move it across
  columns" behavior is reachable via the `m` rune.
- **Issue-aware navigation** — arrow keys skip empty columns and
  snap to actual issue rows rather than walking through empty
  space. Up/Down moves between issues in the current column;
  Left/Right jumps to the first issue of the next non-empty column.
- **Faster `--board` startup** — `openBoardDirect` passes the
  already-fetched `BoardConfiguration` through to the goto handler
  so it doesn't refetch. `GetFilter` is deferred to when actually
  needed (kanban / no active sprint), saving an unconditional 8s
  round trip on scrum boards.
- **Bug fixes** — board scrolling crash when paging past the last
  column; first column not scrolling back into view after navigating
  right then left; F2 filter being silently cleared on `Init` re-entry;
  100% CPU spin from a busy-wait `default:` clause in `handleActions`;
  concurrent-map-write race in `app.Color()` (now `sync.Once`).

### Issues list

- **Numeric query expands to issue-key prefix match** — when a
  project is selected and the query is purely numeric, the search
  uses `key ~ "PROJ-N*"` directly. Typing `53` in COINS matches
  COINS-53, COINS-530–539, COINS-5300, etc. Doesn't false-match
  COINS-153 or descriptions containing "53".
- **F7/F8 exclude-status filter** — F7 stacks status exclusions
  (multi-select), F8 (only visible while exclusions exist) clears
  them all. Current excludes show in the top bar as
  `Exclude Status: -Done, -Won't Fix`.
- **F6 Create Issue** — F6 from the board, issues list, or issue
  detail opens Jira's create-issue modal in your browser with
  project (and board, where applicable) context pre-populated.
- **FuzzyFind opt-in "clear on Esc"** — for the issues fuzzy-find
  only, first Esc clears the query (typo correction without losing
  project context). Second Esc on empty query still backs out.
  Ctrl-C still exits immediately. Project / workspace pickers are
  unaffected.

### Issue detail

- **PgUp / PgDn scrolling** — page through long descriptions or
  comment threads; complements the existing Up/Down/Tab/Backtab
  one-line scrolling.
- **`d` edits the description in-app** — opens a text-writer modal
  pre-populated with the current description. F1 saves via a new
  `DoUpdateDescription` API method that PUTs to
  `/rest/api/2/issue/<id>`. Esc cancels.
- **Text writer cursor positioning** — the editor is now a proper
  in-place editor with arrow-key navigation, Home/End, Shift
  modifiers for larger jumps, Delete/Backspace at cursor, mid-text
  insertion. Useful for both descriptions and comments.

### Atlassian Cloud compatibility

- **Bounded JQL default** — Atlassian Cloud's new
  `/rest/api/3/search/jql` endpoint rejects unbounded queries.
  When no project/status/user/label/query is set, fjira now falls
  back to `created >= -30d ORDER BY status` instead of emitting
  an empty restriction set that 400s. On-prem Server still works
  identically.
- **Expired-token hint** — Atlassian Cloud returns `200` + empty
  project list for invalid/expired API tokens rather than `401`.
  When the projects list is empty and the workspace URL is a
  `*.atlassian.net` host, the flash hint points users at
  `https://id.atlassian.com/manage-profile/security/api-tokens`.

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
