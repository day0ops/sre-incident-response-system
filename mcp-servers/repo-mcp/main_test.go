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
	token := jwtWithPayload(t, `{"sub":"alice","roles":["deployment.rollback"]}`)
	claims, err := decodeJWTClaims("Bearer " + token)
	if err != nil {
		t.Fatalf("decodeJWTClaims() error = %v", err)
	}
	if claims["sub"] != "alice" {
		t.Errorf("claims[sub] = %v; want alice", claims["sub"])
	}
}

func TestWhoamiTool(t *testing.T) {
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
}

func TestRollbackDeploymentTool(t *testing.T) {
	_, out, err := rollbackDeploymentTool(context.Background(), &mcp.CallToolRequest{}, RollbackDeploymentInput{
		Deployment: "checkout-api",
		Version:    "v41",
	})
	if err != nil {
		t.Fatalf("rollbackDeploymentTool() error = %v", err)
	}
	if out.Result.Deployment != "checkout-api" {
		t.Errorf("Result.Deployment = %q; want checkout-api", out.Result.Deployment)
	}
	if out.Result.RolledBackTo != "v41" {
		t.Errorf("Result.RolledBackTo = %q; want v41", out.Result.RolledBackTo)
	}
}
