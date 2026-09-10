package main

import (
	"context"
	"encoding/base64"
	"net/http"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// jwtWithPayload builds a syntactically valid unsigned JWT wrapping the given raw
// JSON payload, for testing decodeJWTClaims.
func jwtWithPayload(t *testing.T, payloadJSON string) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(payloadJSON))
	return header + "." + payload + ".sig"
}

func TestDecodeJWTClaims(t *testing.T) {
	t.Run("valid bearer JWT", func(t *testing.T) {
		token := jwtWithPayload(t, `{"sub":"alice","iss":"https://login.microsoftonline.com/test-tenant/v2.0"}`)
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

	t.Run("malformed base64 payload", func(t *testing.T) {
		_, err := decodeJWTClaims("Bearer aaa.!!!not-base64!!!.ccc")
		if err == nil {
			t.Fatal("decodeJWTClaims() error = nil; want error for bad base64")
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
		if out.Claims["sub"] != "alice" {
			t.Errorf("Claims[sub] = %v; want alice", out.Claims["sub"])
		}
	})
}

func TestGetUnhealthyPodsTool(t *testing.T) {
	_, out, err := getUnhealthyPodsTool(context.Background(), &mcp.CallToolRequest{}, GetUnhealthyPodsInput{})
	if err != nil {
		t.Fatalf("getUnhealthyPodsTool() error = %v", err)
	}
	if len(out.Pods) != len(mockPods) {
		t.Fatalf("got %d pods, want %d", len(out.Pods), len(mockPods))
	}
	for _, p := range out.Pods {
		if p.Ready {
			t.Errorf("pod %s reported ready=true; want an unhealthy pod", p.Pod)
		}
	}
}
