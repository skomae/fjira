package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mk-5/fjira/internal/jira"
	"github.com/mk-5/fjira/internal/workspaces"
	"github.com/spf13/cobra"
)

func GetMcpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Start MCP server for Jira issue querying",
		Long: `Start a Model Context Protocol (MCP) server that allows AI tools to query Jira issues.
The server exposes tools for fetching issue details and comments via the MCP protocol.`,
		Run: func(cmd *cobra.Command, args []string) {
			s := cmd.Context().Value(CtxWorkspaceSettings).(*workspaces.WorkspaceSettings)

			// Initialize Jira API client
			api, err := jira.NewApi(s.JiraRestUrl, s.JiraUsername, s.JiraToken, s.JiraTokenType)
			if err != nil {
				fmt.Printf("Error initializing Jira API: %v\n", err)
				return
			}
			defer api.Close()

			// Create MCP server
			mcpServer := server.NewMCPServer(
				"fjira-mcp-server",
				"1.0.0",
			)

			// Add get_issue tool
			getIssueTool := mcp.NewTool(
				"get_issue",
				mcp.WithDescription("Get detailed information about a Jira issue including description and comments"),
				mcp.WithString("issue_key",
					mcp.Required(),
					mcp.Description("The Jira issue key (e.g., PROJ-123)"),
				),
			)
			mcpServer.AddTool(getIssueTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return getIssueHandler(ctx, request, api)
			})

			// Add search_issues tool
			searchIssuesTool := mcp.NewTool(
				"search_issues",
				mcp.WithDescription("Search for Jira issues using JQL (Jira Query Language)"),
				mcp.WithString("jql",
					mcp.Required(),
					mcp.Description("JQL query string to search for issues"),
				),
				mcp.WithNumber("max_results",
					mcp.Description("Maximum number of results to return (default: 10)"),
				),
			)
			mcpServer.AddTool(searchIssuesTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return searchIssuesHandler(ctx, request, api)
			})

			// Start the stdio server
			fmt.Println("Starting fjira MCP server...")
			if err := server.ServeStdio(mcpServer); err != nil {
				fmt.Printf("MCP server error: %v\n", err)
			}
		},
	}
}

// getIssueHandler handles the get_issue tool call
func getIssueHandler(ctx context.Context, request mcp.CallToolRequest, api jira.Api) (*mcp.CallToolResult, error) {
	arguments := request.GetArguments()
	issueKey, ok := arguments["issue_key"].(string)
	if !ok {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: "Error: issue_key must be a string",
				},
			},
			IsError: true,
		}, nil
	}

	// Validate issue key format
	issueKey = strings.TrimSpace(issueKey)
	if issueKey == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: "Error: issue_key cannot be empty",
				},
			},
			IsError: true,
		}, nil
	}

	// Fetch issue details
	issue, err := api.GetIssueDetailed(issueKey)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Failed to fetch issue %s: %v", issueKey, err),
				},
			},
			IsError: true,
		}, nil
	}

	// Format the response
	response := formatIssueWithComments(issue)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: response,
			},
		},
	}, nil
}

// searchIssuesHandler handles the search_issues tool call
func searchIssuesHandler(ctx context.Context, request mcp.CallToolRequest, api jira.Api) (*mcp.CallToolResult, error) {
	arguments := request.GetArguments()
	jqlQuery, ok := arguments["jql"].(string)
	if !ok {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: "Error: jql must be a string",
				},
			},
			IsError: true,
		}, nil
	}

	jqlQuery = strings.TrimSpace(jqlQuery)
	if jqlQuery == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: "Error: jql query cannot be empty",
				},
			},
			IsError: true,
		}, nil
	}

	// Get max_results parameter (optional)
	maxResults := int32(10) // default
	if maxResultsRaw, exists := arguments["max_results"]; exists {
		if maxResultsFloat, ok := maxResultsRaw.(float64); ok {
			maxResults = int32(maxResultsFloat)
			if maxResults <= 0 || maxResults > 100 {
				maxResults = 10 // reset to default if invalid
			}
		}
	}

	// Search for issues
	issues, total, _, err := api.SearchJqlPageable(jqlQuery, 0, maxResults)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Failed to search issues: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	// Format the response
	response := formatSearchResults(issues, total, jqlQuery)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: response,
			},
		},
	}, nil
}

// formatIssueWithComments formats a detailed issue for display
func formatIssueWithComments(issue *jira.Issue) string {
	var result strings.Builder

	result.WriteString(fmt.Sprintf("# Issue: %s\n\n", issue.Key))
	result.WriteString(fmt.Sprintf("**Summary:** %s\n\n", issue.Fields.Summary))
	result.WriteString(fmt.Sprintf("**Status:** %s\n", issue.Fields.Status.Name))
	result.WriteString(fmt.Sprintf("**Type:** %s\n", issue.Fields.Type.Name))
	result.WriteString(fmt.Sprintf("**Reporter:** %s\n", issue.Fields.Reporter.DisplayName))

	if issue.Fields.Assignee.DisplayName != "" {
		result.WriteString(fmt.Sprintf("**Assignee:** %s\n", issue.Fields.Assignee.DisplayName))
	} else {
		result.WriteString("**Assignee:** Unassigned\n")
	}

	result.WriteString(fmt.Sprintf("**Project:** %s (%s)\n\n", issue.Fields.Project.Name, issue.Fields.Project.Key))

	if len(issue.Fields.Labels) > 0 {
		result.WriteString(fmt.Sprintf("**Labels:** %s\n\n", strings.Join(issue.Fields.Labels, ", ")))
	}

	if issue.Fields.Description != "" {
		result.WriteString("## Description\n\n")
		result.WriteString(fmt.Sprintf("%s\n\n", issue.Fields.Description))
	}

	// Add comments if available
	if len(issue.Fields.Comment.Comments) > 0 {
		result.WriteString("## Comments\n\n")
		for i, comment := range issue.Fields.Comment.Comments {
			result.WriteString(fmt.Sprintf("**Comment %d** by %s (%s):\n",
				i+1, comment.Author.DisplayName, comment.Created))
			result.WriteString(fmt.Sprintf("%s\n\n", comment.Body))
		}
	} else {
		result.WriteString("## Comments\n\nNo comments available.\n\n")
	}

	return result.String()
}

// formatSearchResults formats search results for display
func formatSearchResults(issues []jira.Issue, total int32, jql string) string {
	var result strings.Builder

	result.WriteString(fmt.Sprintf("# Search Results\n\n"))
	result.WriteString(fmt.Sprintf("**JQL Query:** %s\n", jql))
	result.WriteString(fmt.Sprintf("**Results:** Showing %d of %d total issues\n\n", len(issues), total))

	if len(issues) == 0 {
		result.WriteString("No issues found matching the query.\n")
		return result.String()
	}

	for _, issue := range issues {
		assignee := "Unassigned"
		if issue.Fields.Assignee.DisplayName != "" {
			assignee = issue.Fields.Assignee.DisplayName
		}

		result.WriteString(fmt.Sprintf("## %s - %s\n", issue.Key, issue.Fields.Summary))
		result.WriteString(fmt.Sprintf("**Status:** %s | **Assignee:** %s | **Type:** %s\n\n",
			issue.Fields.Status.Name, assignee, issue.Fields.Type.Name))
	}

	return result.String()
}
