// Command repo-mcp is a real MCP server exposing rollback_deployment. It
// performs no JWT verification or role checking of its own -- Enterprise
// Agentgateway is the enforcement point: JWT validation happens against
// Entra directly (this server's trust domain, unlike incident-mcp/runbook-mcp
// which sit behind Keycloak), and the deployment.rollback role requirement is
// enforced by agentgateway's own mcp.authorization CEL policy before the call
// ever reaches this server.
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

const serverName = "repo-mcp"

type RollbackDeploymentInput struct {
	Deployment string `json:"deployment" jsonschema:"the deployment to roll back"`
	Version    string `json:"version" jsonschema:"the version to roll back to"`
}

type RollbackDeploymentOutput struct {
	Result RollbackResult `json:"result" jsonschema:"the outcome of the rollback"`
}

func rollbackDeploymentTool(_ context.Context, _ *mcp.CallToolRequest, in RollbackDeploymentInput) (*mcp.CallToolResult, RollbackDeploymentOutput, error) {
	return nil, RollbackDeploymentOutput{Result: mockRollback(in.Deployment, in.Version)}, nil
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
		Name:        "rollback_deployment",
		Description: "Roll back a deployment to a previous version",
	}, rollbackDeploymentTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "whoami",
		Description: "Diagnostic: decode the Authorization header this server actually received, to prove the Entra OBO exchange happened",
	}, whoamiTool)

	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, nil)

	mux := http.NewServeMux()
	mux.Handle("/mcp", handler)

	addr := ":" + envOr("PORT", "9103")
	log.Printf("%s MCP server listening on %s (POST /mcp)", serverName, addr)
	if err := http.ListenAndServe(addr, mux); err != nil { //nolint:gosec // demo server, no custom timeouts needed
		log.Fatalf("server error: %v", err)
	}
}
