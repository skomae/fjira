# Testing fjira MCP Server

This guide shows how to test the fjira MCP server functionality.

## Prerequisites

1. **Build fjira with MCP support:**
   ```bash
   go build -o fjira-mcp cmd/fjira-cli/main.go
   ```

2. **Configure a fjira workspace** (if not already done):
   ```bash
   ./fjira-mcp workspace
   ```

## Method 1: Using MCP Inspector (Recommended)

MCP Inspector is a web-based testing tool for MCP servers.

### Installation and Usage

1. **Install MCP Inspector** (requires Node.js):
   ```bash
   npx @modelcontextprotocol/inspector ./fjira-mcp mcp
   ```

2. **Open the web interface:**
   - The command will output a URL like `http://127.0.0.1:6274`
   - Open this URL in your browser

3. **Test the tools:**
   
   **Test get_issue tool:**
   - Select "get_issue" from the tools list
   - Enter an issue key (e.g., "PROJ-123") in the `issue_key` parameter
   - Click "Call Tool"
   - Verify you get detailed issue information

   **Test search_issues tool:**
   - Select "search_issues" from the tools list
   - Enter a JQL query (e.g., "project = MYPROJ") in the `jql` parameter
   - Optionally set `max_results` (e.g., 5)
   - Click "Call Tool"
   - Verify you get a list of matching issues

## Method 2: Manual Testing with AI Clients

### Claude Desktop Testing

1. **Configure Claude Desktop** (see main documentation)

2. **Test queries in Claude:**
   ```
   User: "Get details for issue ABC-123"
   User: "Find all open issues in project XYZ"
   User: "Show me bugs assigned to john.doe@company.com"
   ```

3. **Verify responses:**
   - Claude should use the fjira tools automatically
   - Check that issue details include description and comments
   - Verify search results show relevant issues

### Cursor IDE Testing

1. **Configure Cursor** (see main documentation)

2. **Test in Cursor chat:**
   - Open a workspace in Cursor
   - Use the chat feature to query Jira issues
   - Verify tool integration works correctly

## Method 3: Direct Command Line Testing

For basic server startup testing:

```bash
# Test that the server starts without errors
./fjira-mcp mcp
# Should output: "Starting fjira MCP server..."
# Then wait for input (Ctrl+C to stop)
```

## Expected Behaviors

### Successful get_issue Response
```markdown
# Issue: ABC-123

**Summary:** Fix login authentication bug

**Status:** In Progress
**Type:** Bug
**Reporter:** jane.smith@company.com
**Assignee:** john.doe@company.com
**Project:** Authentication System (AUTH)

**Labels:** security, critical

## Description

Users are unable to log in with valid credentials...

## Comments

**Comment 1** by john.doe@company.com (2024-01-15T10:30:00.000+0000):
Started investigating this issue...

**Comment 2** by jane.smith@company.com (2024-01-15T14:20:00.000+0000):
This affects 15% of our users...
```

### Successful search_issues Response
```markdown
# Search Results

**JQL Query:** project = AUTH AND status = "In Progress"
**Results:** Showing 3 of 15 total issues

## AUTH-123 - Fix login authentication bug
**Status:** In Progress | **Assignee:** john.doe@company.com | **Type:** Bug

## AUTH-124 - Update password validation
**Status:** In Progress | **Assignee:** jane.smith@company.com | **Type:** Task

## AUTH-125 - Add two-factor authentication
**Status:** In Progress | **Assignee:** Unassigned | **Type:** Story
```

## Common Issues and Solutions

### Issue: "Error initializing Jira API"
**Solution:** Configure workspace credentials:
```bash
./fjira-mcp workspace
```

### Issue: "Failed to fetch issue"
**Causes:**
- Issue key doesn't exist
- No permission to view the issue
- Network connectivity issues

**Solution:** Verify the issue exists and you have access

### Issue: "Invalid JQL query"
**Causes:**
- Syntax error in JQL
- Invalid field names
- Invalid operators

**Solution:** Test JQL queries in Jira web interface first

### Issue: MCP Inspector shows no tools
**Causes:**
- Server failed to start
- Network/connection issues
- Invalid configuration

**Solution:** Check server logs and restart

## Performance Notes

- **get_issue** calls: ~200-500ms per issue (depending on network)
- **search_issues** calls: ~300-1000ms per query (depending on result count)
- Results are formatted for optimal AI consumption
- No caching implemented - each call hits Jira API directly

## Security Verification

Verify that the MCP server:
1. ✅ Only exposes read-only operations
2. ✅ Validates input parameters
3. ✅ Uses existing workspace authentication
4. ✅ Does not expose credentials to clients
5. ✅ Handles errors gracefully without exposing sensitive info

## Next Steps

Once testing is successful:
1. Configure your preferred AI client (Claude Desktop, Cursor, etc.)
2. Share the configuration with your team
3. Train users on effective prompting techniques
4. Monitor usage and performance 