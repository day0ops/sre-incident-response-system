package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type echoInput struct {
	Name string `json:"name"`
}

type echoOutput struct {
	Greeting string `json:"greeting"`
}

func echoTool(_ context.Context, _ *mcp.CallToolRequest, in echoInput) (*mcp.CallToolResult, echoOutput, error) {
	return nil, echoOutput{Greeting: "hello " + in.Name}, nil
}

// TestCallToolJSONPropagatesBearerToken exercises the real MCP
// streamable-HTTP protocol round-trip (client -> server, both from this SDK)
// rather than mocking it, and confirms the bearer token this agent holds
// actually reaches the server.
func TestCallToolJSONPropagatesBearerToken(t *testing.T) {
	var gotAuthHeader string

	server := mcp.NewServer(&mcp.Implementation{Name: "echo-test", Version: "0.1.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "echo", Description: "echoes a greeting"}, echoTool)
	handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		gotAuthHeader = r.Header.Get("Authorization")
		return server
	}, nil)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	raw, err := callToolJSON(context.Background(), srv.URL, "test-token", "echo", echoInput{Name: "alice"})
	if err != nil {
		t.Fatalf("callToolJSON() error = %v", err)
	}
	if raw != `{"greeting":"hello alice"}` {
		t.Fatalf("got raw JSON %q", raw)
	}
	if gotAuthHeader != "Bearer test-token" {
		t.Fatalf("server received Authorization header %q, want %q", gotAuthHeader, "Bearer test-token")
	}
}

func TestListTools(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "echo-test", Version: "0.1.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "echo", Description: "echoes a greeting"}, echoTool)
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	tools, err := listTools(context.Background(), srv.URL, "test-token")
	if err != nil {
		t.Fatalf("listTools() error = %v", err)
	}
	if len(tools) != 1 || tools[0].Name != "echo" {
		t.Fatalf("got tools %+v, want one tool named echo", tools)
	}
	if tools[0].InputSchema == nil {
		t.Fatal("expected a non-nil InputSchema")
	}
}

func TestCallToolJSON(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "echo-test", Version: "0.1.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "echo", Description: "echoes a greeting"}, echoTool)
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	raw, err := callToolJSON(context.Background(), srv.URL, "test-token", "echo", echoInput{Name: "bob"})
	if err != nil {
		t.Fatalf("callToolJSON() error = %v", err)
	}
	if raw != `{"greeting":"hello bob"}` {
		t.Fatalf("got raw JSON %q", raw)
	}
}
