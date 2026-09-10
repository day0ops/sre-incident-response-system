package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChatCompletion(t *testing.T) {
	t.Run("plain text response", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req chatCompletionRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			if req.Model != "gpt-4o" {
				t.Fatalf("unexpected model: %s", req.Model)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []map[string]any{
					{"message": map[string]any{"role": "assistant", "content": "all good"}, "finish_reason": "stop"},
				},
			})
		}))
		defer srv.Close()

		msg, err := chatCompletion(context.Background(), srv.URL, "gpt-4o", []chatMessage{{Role: "user", Content: "hi"}}, nil)
		if err != nil {
			t.Fatalf("chatCompletion() error = %v", err)
		}
		if msg.Content != "all good" {
			t.Fatalf("got content %q", msg.Content)
		}
	})

	t.Run("tool call response", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []map[string]any{
					{
						"message": map[string]any{
							"role": "assistant",
							"tool_calls": []map[string]any{
								{"id": "call_1", "type": "function", "function": map[string]any{"name": "get_unhealthy_pods", "arguments": "{}"}},
							},
						},
						"finish_reason": "tool_calls",
					},
				},
			})
		}))
		defer srv.Close()

		msg, err := chatCompletion(context.Background(), srv.URL, "gpt-4o", []chatMessage{{Role: "user", Content: "investigate"}}, nil)
		if err != nil {
			t.Fatalf("chatCompletion() error = %v", err)
		}
		if len(msg.ToolCalls) != 1 || msg.ToolCalls[0].Function.Name != "get_unhealthy_pods" {
			t.Fatalf("got tool calls %+v", msg.ToolCalls)
		}
	})

	t.Run("non-200 response is an error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
		}))
		defer srv.Close()

		_, err := chatCompletion(context.Background(), srv.URL, "gpt-4o", []chatMessage{{Role: "user", Content: "hi"}}, nil)
		if err == nil {
			t.Fatal("expected an error, got nil")
		}
	})
}
