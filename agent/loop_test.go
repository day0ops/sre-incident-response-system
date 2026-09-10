package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type pingInput struct{}
type pingOutput struct {
	Status string `json:"status"`
}

func pingTool(_ context.Context, _ *mcp.CallToolRequest, _ pingInput) (*mcp.CallToolResult, pingOutput, error) {
	return nil, pingOutput{Status: "ok"}, nil
}

func startTestMCPServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "test-mcp", Version: "0.1.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "ping", Description: "pings"}, pingTool)
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)
	return httptest.NewServer(handler)
}

func TestRunLoopFinalAnswerNoTools(t *testing.T) {
	llm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"role": "assistant", "content": "nothing to report"}, "finish_reason": "stop"},
			},
		})
	}))
	defer llm.Close()

	result := runLoop(context.Background(), llm.URL, "gpt-4o", "token", "http://unused", nil, nil,
		[]chatMessage{{Role: "user", Content: "check things"}})

	if result.FinalText != "nothing to report" {
		t.Fatalf("got FinalText %q", result.FinalText)
	}
	if result.ConsentURL != "" {
		t.Fatalf("expected no consent URL, got %q", result.ConsentURL)
	}
}

func TestRunLoopSingleToolCallThenAnswer(t *testing.T) {
	mcpSrv := startTestMCPServer(t)
	defer mcpSrv.Close()

	var calls int32
	llm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []map[string]any{
					{
						"message": map[string]any{
							"role": "assistant",
							"tool_calls": []map[string]any{
								{"id": "call_1", "type": "function", "function": map[string]any{"name": "ping", "arguments": "{}"}},
							},
						},
						"finish_reason": "tool_calls",
					},
				},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"role": "assistant", "content": "ping succeeded"}, "finish_reason": "stop"},
			},
		})
	}))
	defer llm.Close()

	toolServer := map[string]string{"ping": mcpSrv.URL}
	result := runLoop(context.Background(), llm.URL, "gpt-4o", "token", "http://unused", toolServer, nil,
		[]chatMessage{{Role: "user", Content: "ping it"}})

	if result.FinalText != "ping succeeded" {
		t.Fatalf("got FinalText %q", result.FinalText)
	}
	foundToolResult := false
	for _, m := range result.Messages {
		if m.Role == "tool" && m.Content == `{"status":"ok"}` {
			foundToolResult = true
		}
	}
	if !foundToolResult {
		t.Fatalf("expected a tool result message in %+v", result.Messages)
	}
}

func TestRunLoopElicitationPause(t *testing.T) {
	// A tool call that always fails at the MCP layer (server never even starts).
	toolServer := map[string]string{"rollback_deployment": "http://127.0.0.1:1"}

	sts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{
				"Elicitation": map[string]any{"ID": 1, "resource": "repo-mcp", "status": "pending"},
				"OAuthConfig": map[string]any{"authorize_url": "https://entra.example/consent"},
			},
		})
	}))
	defer sts.Close()

	llm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"role": "assistant",
						"tool_calls": []map[string]any{
							{"id": "call_1", "type": "function", "function": map[string]any{"name": "rollback_deployment", "arguments": `{"deployment":"checkout-api","version":"v41"}`}},
						},
					},
					"finish_reason": "tool_calls",
				},
			},
		})
	}))
	defer llm.Close()

	result := runLoop(context.Background(), llm.URL, "gpt-4o", "token", sts.URL, toolServer, nil,
		[]chatMessage{{Role: "user", Content: "roll it back"}})

	if result.ConsentURL != "https://entra.example/consent" {
		t.Fatalf("got ConsentURL %q", result.ConsentURL)
	}
	if result.FinalText != "" {
		t.Fatalf("expected no final text, got %q", result.FinalText)
	}
	if result.PendingCall.Function.Name != "rollback_deployment" {
		t.Fatalf("got PendingCall %+v", result.PendingCall)
	}
	if result.PendingServer != "http://127.0.0.1:1" {
		t.Fatalf("got PendingServer %q", result.PendingServer)
	}
}

func TestResumeLoop(t *testing.T) {
	mcpSrv := startTestMCPServer(t)
	defer mcpSrv.Close()

	llm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"role": "assistant", "content": "done"}, "finish_reason": "stop"},
			},
		})
	}))
	defer llm.Close()

	pending := toolCall{ID: "call_1", Type: "function", Function: functionCall{Name: "ping", Arguments: "{}"}}
	messages := []chatMessage{
		{Role: "user", Content: "ping it"},
		{Role: "assistant", ToolCalls: []toolCall{pending}},
	}

	result := resumeLoop(context.Background(), llm.URL, "gpt-4o", "token", "http://unused", map[string]string{"ping": mcpSrv.URL}, nil, messages, pending, mcpSrv.URL)

	if result.FinalText != "done" {
		t.Fatalf("got FinalText %q", result.FinalText)
	}
	foundToolResult := false
	for _, m := range result.Messages {
		if m.Role == "tool" && m.ToolCallID == "call_1" && m.Content == `{"status":"ok"}` {
			foundToolResult = true
		}
	}
	if !foundToolResult {
		t.Fatalf("expected the retried tool's result in %+v", result.Messages)
	}
}
