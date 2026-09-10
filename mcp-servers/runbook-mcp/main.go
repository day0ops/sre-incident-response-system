// Command runbook-mcp is a real MCP server exposing get_deployment_history
// over the mock data in mockdata.go. Same Keycloak realm as incident-mcp but a
// different audience, so a token minted for one is rejected by the other --
// this server's role in the design is proving that audience isolation. It
// doesn't verify JWT signatures itself -- Enterprise Agentgateway does that --
// and it doesn't gate get_deployment_history itself either: that's enforced
// entirely at the gateway (a CEL policy on the exchanged token's scope claim,
// which Keycloak's downscope-role-enforcer caps to the caller's Entra
// app-role assignment), matching the "trust the gateway" decision this
// design already makes for repo-mcp's rollback_deployment.
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const serverName = "runbook-mcp"

type GetDeploymentHistoryInput struct {
	Deployment string `json:"deployment,omitempty" jsonschema:"the deployment to check, defaults to checkout-api"`
}

type GetDeploymentHistoryOutput struct {
	Deployments []Deployment `json:"deployments" jsonschema:"recent deployment history"`
}

func getDeploymentHistoryTool(_ context.Context, _ *mcp.CallToolRequest, _ GetDeploymentHistoryInput) (*mcp.CallToolResult, GetDeploymentHistoryOutput, error) {
	return nil, GetDeploymentHistoryOutput{Deployments: mockDeployments}, nil
}

type WhoamiOutput struct {
	AuthorizationPresent bool           `json:"authorization_present" jsonschema:"whether an Authorization header was received on this request"`
	Claims               map[string]any `json:"claims,omitempty" jsonschema:"decoded (unverified) JWT claims from the received token, if present and well-formed"`
}

func whoamiTool(_ context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, WhoamiOutput, error) {
	if req.Extra == nil || req.Extra.Header == nil {
		return nil, WhoamiOutput{}, nil
	}
	authHeader := req.Extra.Header.Get("Authorization")
	if authHeader == "" {
		return nil, WhoamiOutput{}, nil
	}
	claims, err := decodeJWTClaims(authHeader)
	if err != nil {
		return nil, WhoamiOutput{AuthorizationPresent: true}, nil
	}
	return nil, WhoamiOutput{AuthorizationPresent: true, Claims: claims}, nil
}

func decodeJWTClaims(authHeader string) (map[string]any, error) {
	token := strings.TrimPrefix(authHeader, "Bearer ")
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("not a JWT: expected 3 dot-separated parts, got %d", len(parts))
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decoding JWT payload: %w", err)
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("parsing JWT claims: %w", err)
	}
	return claims, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	server := mcp.NewServer(&mcp.Implementation{Name: serverName, Version: "0.1.0"}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_deployment_history",
		Description: "List recent deployment versions and when they were deployed",
	}, getDeploymentHistoryTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "whoami",
		Description: "Diagnostic: decode the Authorization header this server actually received, to prove token exchange happened",
	}, whoamiTool)

	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, nil)

	mux := http.NewServeMux()
	mux.Handle("/mcp", handler)

	addr := ":" + envOr("PORT", "9102")
	log.Printf("%s MCP server listening on %s (POST /mcp)", serverName, addr)
	if err := http.ListenAndServe(addr, mux); err != nil { //nolint:gosec // demo server, no custom timeouts needed
		log.Fatalf("server error: %v", err)
	}
}
