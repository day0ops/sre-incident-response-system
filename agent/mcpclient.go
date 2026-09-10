// mcpclient.go wraps the real MCP client (modelcontextprotocol/go-sdk) with
// this agent's bearer-token propagation. Every call carries the caller's
// Entra JWT as-is; Enterprise Agentgateway (not this agent) decides whether
// that means a Keycloak exchange or an Entra On-Behalf-Of call before the
// request reaches the target MCP server.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type bearerRoundTripper struct {
	token string
	base  http.RoundTripper
}

func (rt *bearerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+rt.token)
	return rt.base.RoundTrip(req)
}

// connect opens a fresh MCP client session against serverURL, carrying
// bearerToken on every request. One connection per call keeps this stateless
// and simple for a low-volume demo agent -- a long-lived session would be the
// right call for anything higher-throughput. Callers must close the session.
func connect(ctx context.Context, serverURL, bearerToken string) (*mcp.ClientSession, error) {
	httpClient := &http.Client{Transport: &bearerRoundTripper{token: bearerToken, base: http.DefaultTransport}}
	client := mcp.NewClient(&mcp.Implementation{Name: "incident-response-agent", Version: "0.1.0"}, nil)

	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: serverURL, HTTPClient: httpClient}, nil)
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", serverURL, err)
	}
	return session, nil
}

// listTools returns serverURL's tool definitions (name, description,
// JSON-schema input) -- fed into the LLM's tool-calling request as-is, rather
// than hand-duplicating each server's tool schema in agent code.
func listTools(ctx context.Context, serverURL, bearerToken string) ([]*mcp.Tool, error) {
	session, err := connect(ctx, serverURL, bearerToken)
	if err != nil {
		return nil, err
	}
	defer func() { _ = session.Close() }()

	result, err := session.ListTools(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("list tools at %s: %w", serverURL, err)
	}
	return result.Tools, nil
}

// callToolJSON calls toolName on serverURL and returns its structured result
// as a raw JSON string -- exactly the shape the LLM loop needs for a "tool"
// role message; the LLM reads JSON text, not Go types.
func callToolJSON(ctx context.Context, serverURL, bearerToken, toolName string, args any) (string, error) {
	session, err := connect(ctx, serverURL, bearerToken)
	if err != nil {
		return "", err
	}
	defer func() { _ = session.Close() }()

	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: toolName, Arguments: args})
	if err != nil {
		return "", fmt.Errorf("call %s: %w", toolName, err)
	}
	if result.IsError {
		return "", fmt.Errorf("tool %s: %s", toolName, toolErrorText(result))
	}

	raw, err := json.Marshal(result.StructuredContent)
	if err != nil {
		return "", fmt.Errorf("marshal structured content from %s: %w", toolName, err)
	}
	return string(raw), nil
}

// toolErrorText extracts the human-readable message a tool set via
// CallToolResult.SetError -- without this, an IsError result's actual reason
// (e.g. "access denied: requires membership in ...") is lost and every
// failure looks identical to the LLM and the chat UI.
func toolErrorText(result *mcp.CallToolResult) string {
	for _, c := range result.Content {
		if text, ok := c.(*mcp.TextContent); ok {
			return text.Text
		}
	}
	return "returned an error result"
}
