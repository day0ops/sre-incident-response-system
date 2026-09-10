// Command incident-mcp is a real MCP server exposing get_unhealthy_pods over
// the mock data in mockdata.go. It performs no JWT verification of its own --
// Enterprise Agentgateway is the enforcement point (JWT auth + token exchange
// happen there), matching this repo's sibling day0ops/retail-returns-agent-system.
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

const serverName = "incident-mcp"

type GetUnhealthyPodsInput struct {
	Namespace string `json:"namespace,omitempty" jsonschema:"the namespace to check, defaults to checkout"`
}

type GetUnhealthyPodsOutput struct {
	Pods []Pod `json:"pods" jsonschema:"unhealthy pods found"`
}

func getUnhealthyPodsTool(_ context.Context, _ *mcp.CallToolRequest, _ GetUnhealthyPodsInput) (*mcp.CallToolResult, GetUnhealthyPodsOutput, error) {
	return nil, GetUnhealthyPodsOutput{Pods: mockPods}, nil
}

type WhoamiOutput struct {
	AuthorizationPresent bool           `json:"authorization_present" jsonschema:"whether an Authorization header was received on this request"`
	Claims               map[string]any `json:"claims,omitempty" jsonschema:"decoded (unverified) JWT claims from the received token, if present and well-formed"`
}

// whoamiTool decodes and returns the claims of the bearer token this server
// actually received, so a live run can confirm agentgateway's token exchange
// happened. No signature verification; display aid, not an auth check --
// enforcement is agentgateway's job, not this server's.
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
		Name:        "get_unhealthy_pods",
		Description: "List unhealthy pods and why they're failing readiness checks",
	}, getUnhealthyPodsTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "whoami",
		Description: "Diagnostic: decode the Authorization header this server actually received, to prove token exchange happened",
	}, whoamiTool)

	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, nil)

	mux := http.NewServeMux()
	mux.Handle("/mcp", handler)

	addr := ":" + envOr("PORT", "9101")
	log.Printf("%s MCP server listening on %s (POST /mcp)", serverName, addr)
	if err := http.ListenAndServe(addr, mux); err != nil { //nolint:gosec // demo server, no custom timeouts needed
		log.Fatalf("server error: %v", err)
	}
}
