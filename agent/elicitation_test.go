package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListElicitations(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer entra-jwt" {
			t.Fatalf("unexpected Authorization header: %s", got)
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{
				"Elicitation": map[string]any{"ID": 7, "resource": "repo-mcp", "status": "pending"},
				"OAuthConfig": map[string]any{
					"client_id":     "agent-gateway",
					"authorize_url": "https://login.microsoftonline.com/tenant/oauth2/v2.0/authorize?client_id=agent-gateway",
					"redirect_uri":  "http://localhost:9200/elicitation/callback",
					"scopes":        []string{"api://repo-mcp/.default"},
				},
			},
		})
	}))
	defer srv.Close()

	elicitations, err := listElicitations(srv.URL, "entra-jwt")
	if err != nil {
		t.Fatalf("listElicitations() error = %v", err)
	}
	if len(elicitations) != 1 {
		t.Fatalf("got %d elicitations, want 1", len(elicitations))
	}
	pending, ok := firstPending(elicitations)
	if !ok {
		t.Fatal("firstPending() ok = false, want true")
	}
	if pending.ID != 7 || pending.Resource != "repo-mcp" {
		t.Fatalf("got %+v, want ID=7 Resource=repo-mcp", pending)
	}
	if pending.OAuth.AuthorizeURL == "" {
		t.Fatal("expected a non-empty AuthorizeURL")
	}
}

func TestFirstPendingNoneAvailable(t *testing.T) {
	_, ok := firstPending([]PendingElicitation{{ID: 1, Status: "completed"}})
	if ok {
		t.Fatal("firstPending() ok = true, want false when nothing is pending")
	}
}

func TestCompleteElicitation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["id"] != float64(7) || body["resource"] != "repo-mcp" || body["status"] != "completed" {
			t.Fatalf("unexpected body: %+v", body)
		}
		oauthConfig, _ := body["oauth_config"].(map[string]any)
		if oauthConfig["code"] != "auth-code-123" {
			t.Fatalf("unexpected oauth_config: %+v", oauthConfig)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	err := completeElicitation(srv.URL, "entra-jwt", PendingElicitation{ID: 7, Resource: "repo-mcp"}, "auth-code-123")
	if err != nil {
		t.Fatalf("completeElicitation() error = %v", err)
	}
}

func TestCompleteElicitationFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	err := completeElicitation(srv.URL, "entra-jwt", PendingElicitation{ID: 7, Resource: "repo-mcp"}, "bad-code")
	if err == nil {
		t.Fatal("completeElicitation() error = nil, want error on non-200 response")
	}
}
