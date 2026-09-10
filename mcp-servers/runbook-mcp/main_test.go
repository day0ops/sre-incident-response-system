package main

import (
	"context"
	"encoding/base64"
	"net/http"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func jwtWithPayload(t *testing.T, payloadJSON string) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(payloadJSON))
	return header + "." + payload + ".sig"
}

func TestDecodeJWTClaims(t *testing.T) {
	t.Run("valid bearer JWT", func(t *testing.T) {
		token := jwtWithPayload(t, `{"sub":"alice"}`)
		claims, err := decodeJWTClaims("Bearer " + token)
		if err != nil {
			t.Fatalf("decodeJWTClaims() error = %v", err)
		}
		if claims["sub"] != "alice" {
			t.Errorf("claims[sub] = %v; want alice", claims["sub"])
		}
	})

	t.Run("not a JWT", func(t *testing.T) {
		_, err := decodeJWTClaims("Bearer not-a-jwt")
		if err == nil {
			t.Fatal("decodeJWTClaims() error = nil; want error for malformed token")
		}
	})
}

func TestWhoamiTool(t *testing.T) {
	t.Run("no Extra on the request", func(t *testing.T) {
		_, out, err := whoamiTool(context.Background(), &mcp.CallToolRequest{}, struct{}{})
		if err != nil {
			t.Fatalf("whoamiTool() error = %v", err)
		}
		if out.AuthorizationPresent {
			t.Error("AuthorizationPresent = true; want false when Extra is nil")
		}
	})

	t.Run("real Authorization header decodes to claims", func(t *testing.T) {
		token := jwtWithPayload(t, `{"sub":"alice"}`)
		header := http.Header{}
		header.Set("Authorization", "Bearer "+token)
		req := &mcp.CallToolRequest{Extra: &mcp.RequestExtra{Header: header}}
		_, out, err := whoamiTool(context.Background(), req, struct{}{})
		if err != nil {
			t.Fatalf("whoamiTool() error = %v", err)
		}
		if !out.AuthorizationPresent {
			t.Error("AuthorizationPresent = false; want true")
		}
	})
}

func TestGetDeploymentHistoryTool(t *testing.T) {
	_, out, err := getDeploymentHistoryTool(context.Background(), &mcp.CallToolRequest{}, GetDeploymentHistoryInput{})
	if err != nil {
		t.Fatalf("getDeploymentHistoryTool() error = %v", err)
	}
	if len(out.Deployments) != len(mockDeployments) {
		t.Fatalf("got %d deployments, want %d", len(out.Deployments), len(mockDeployments))
	}
}
