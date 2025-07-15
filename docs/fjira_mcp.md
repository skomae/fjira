# fjira mcp

Start a Model Context Protocol (MCP) server for Jira issue querying

## Synopsis

Start a Model Context Protocol (MCP) server that allows AI tools to query Jira issues.
The server exposes tools for fetching issue details and comments via the MCP protocol.

This enables AI applications like Claude Desktop, Cursor, and other MCP-compatible tools
to directly query your Jira instance through fjira.

```
fjira mcp [flags]
```

## Description

The MCP server exposes the following tools:

### get_issue

Fetches detailed information about a specific Jira issue, including:
- Issue summary, description, and metadata
- Current status and assignee
- All comments with authors and timestamps
- Labels and project information

**Parameters:**
- `issue_key` (required): The Jira issue key (e.g., "PROJ-123")

### search_issues

Searches for Jira issues using JQL (Jira Query Language):
- Supports full JQL syntax for advanced filtering
- Returns formatted issue summaries
- Configurable result limits

**Parameters:**
- `jql` (required): JQL query string (e.g., "project = MYPROJ AND status = Open")
- `max_results` (optional): Maximum number of results to return (default: 10, max: 100)

## Setup

### Prerequisites

1. **Jira Workspace Configuration**: You must have a configured fjira workspace with valid Jira credentials.
   ```bash
   fjira workspace
   ```

2. **MCP-Compatible Client**: An AI application that supports the Model Context Protocol, such as:
   - Claude Desktop
   - Cursor IDE
   - VS Code with MCP extension
   - Custom MCP clients

### Claude Desktop Configuration

1. Install and configure fjira with your Jira credentials
2. Build the fjira binary: `go build -o fjira cmd/fjira-cli/main.go`
3. Edit your Claude Desktop configuration file (`claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "fjira": {
      "command": "/path/to/your/fjira",
      "args": ["mcp"],
      "env": {}
    }
  }
}
```

4. Restart Claude Desktop
5. The fjira tools will now be available in Claude conversations

### Cursor IDE Configuration

1. Open Cursor IDE settings
2. Navigate to the MCP section
3. Add a new MCP server with:
   - **Name**: fjira
   - **Command**: `/path/to/your/fjira`
   - **Args**: `["mcp"]`

## Usage Examples

Once configured, you can ask your AI assistant to:

### Get Issue Details
> "Get me the details for issue PROJ-123"

The AI will use the `get_issue` tool to fetch comprehensive information about the issue.

### Search Issues
> "Find all open issues assigned to me in the MARKETING project"

The AI will construct an appropriate JQL query using the `search_issues` tool.

### Advanced Queries
> "Show me all bugs with high priority created in the last week"

The AI can construct complex JQL queries for sophisticated issue searching.

## Options

```
  -h, --help   help for mcp
```

## Notes

- The server runs until terminated (Ctrl+C)
- All Jira API calls use your configured workspace credentials
- The server communicates via stdio (standard input/output) as per MCP specification
- Error handling includes validation of issue keys and JQL syntax

## Security

The MCP server:
- Only exposes read-only operations (fetching and searching issues)
- Uses your existing fjira workspace authentication
- Does not expose sensitive credentials to AI clients
- Validates all input parameters to prevent injection attacks

## Troubleshooting

### "Error initializing Jira API"
Ensure you have a properly configured fjira workspace:
```bash
fjira workspace
```

### "Connection refused" or similar network errors
Verify your Jira instance is accessible and credentials are valid:
```bash
fjira --help  # Test basic functionality
```

### MCP client not detecting tools
1. Verify the fjira binary path in your MCP client configuration
2. Check that the fjira binary has execute permissions
3. Restart your MCP client application

## SEE ALSO

* [fjira](fjira.md) - Main fjira command
* [fjira workspace](fjira_workspace.md) - Workspace management
* [Model Context Protocol Documentation](https://modelcontextprotocol.io/) 