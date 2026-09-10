// main.go
package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
)

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

const accessDeniedMessage = "agent: access denied - no tools available for this account"

type taskRequest struct {
	Task string `json:"task"`
}

type taskResponse struct {
	Steps      []string `json:"steps"`
	Result     any      `json:"result,omitempty"`
	ConsentURL string   `json:"consent_url,omitempty"`
}

// pendingLoop remembers the conversation an elicitation-gated tool call
// paused, so a later completion request (once the user finishes the OAuth
// consent) can retry that exact call and continue the same LLM loop. Single
// in-memory slot -- fine for a low-volume demo agent with one user at a time,
// matching chat-app's own in-memory session store.
type pendingLoop struct {
	mu       sync.Mutex
	messages []chatMessage
	call     toolCall
	server   string
}

var lastPending pendingLoop

func savePending(messages []chatMessage, call toolCall, server string) {
	lastPending.mu.Lock()
	defer lastPending.mu.Unlock()
	lastPending.messages = messages
	lastPending.call = call
	lastPending.server = server
}

func loadPending() ([]chatMessage, toolCall, string) {
	lastPending.mu.Lock()
	defer lastPending.mu.Unlock()
	return lastPending.messages, lastPending.call, lastPending.server
}

func main() {
	entraJWKSURL := envOr("ENTRA_JWKS_URL", "")
	entraIssuer := envOr("ENTRA_ISSUER", "")
	entraAudience := envOr("ENTRA_AUDIENCE", "")
	incidentMCPURL := envOr("INCIDENT_MCP_URL", "http://localhost:9101/mcp")
	runbookMCPURL := envOr("RUNBOOK_MCP_URL", "http://localhost:9102/mcp")
	repoMCPURL := envOr("REPO_MCP_URL", "http://localhost:9103/mcp")
	stsURL := envOr("STS_URL", "http://enterprise-agentgateway.agentgateway-system.svc.cluster.local:7777")
	// agentgateway's own OpenAI-compatible route (see this repo's providers
	// feature) - this agent never holds a provider API key itself.
	llmURL := envOr("LLM_URL", "http://localhost:9100/sre-irs/llm/v1")
	llmModel := envOr("LLM_MODEL", "gpt-4o")
	serverURLs := []string{incidentMCPURL, runbookMCPURL, repoMCPURL}

	http.HandleFunc("/task", func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if _, err := verifyBearer(r, entraJWKSURL, entraIssuer, entraAudience); err != nil {
			log.Printf("agent rejected its own token: %v", err)
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(taskResponse{Steps: []string{"agent: token validation failed: " + err.Error()}})
			return
		}
		bearer := authHeader[len("Bearer "):]

		var req taskRequest
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)

		ctx := r.Context()
		tools, toolServer, err := discoverTools(ctx, serverURLs, bearer)
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(taskResponse{Steps: []string{"agent: could not discover tools: " + err.Error()}})
			return
		}
		// discoverTools skips servers it can't reach rather than failing the whole call
		// (see loop.go) -- if every server rejected this bearer, that's not "no servers
		// happened to be up," it's this identity having no tool access at all (e.g. no
		// federated-identity link in Keycloak yet), so it's surfaced distinctly rather
		// than silently handing the LLM an empty tool list.
		if len(tools) == 0 {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(taskResponse{Steps: []string{accessDeniedMessage}, Result: accessDeniedMessage})
			return
		}

		messages := []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: req.Task},
		}
		result := runLoop(ctx, llmURL, llmModel, bearer, stsURL, toolServer, tools, messages)

		if result.ConsentURL != "" {
			savePending(result.Messages, result.PendingCall, result.PendingServer)
			_ = json.NewEncoder(w).Encode(taskResponse{Steps: result.Steps, ConsentURL: result.ConsentURL})
			return
		}
		_ = json.NewEncoder(w).Encode(taskResponse{Steps: result.Steps, Result: result.FinalText})
	})

	// Completes a pending tool call's consent (see elicitation.go): the caller
	// hands back the OAuth authorization code obtained from the consent
	// redirect, the STS exchanges it for a real token, the gated call is
	// retried, and the same LLM loop continues from there.
	http.HandleFunc("/task/complete-rollback", func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if _, err := verifyBearer(r, entraJWKSURL, entraIssuer, entraAudience); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(taskResponse{Steps: []string{"agent: token validation failed: " + err.Error()}})
			return
		}
		bearer := authHeader[len("Bearer "):]

		var req struct {
			Code string `json:"code"`
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)

		ctx := r.Context()
		steps := []string{"agent: completing rollback consent"}

		elicitations, err := listElicitations(stsURL, bearer)
		if err != nil {
			steps = append(steps, "agent: could not list elicitations: "+err.Error())
			_ = json.NewEncoder(w).Encode(taskResponse{Steps: steps})
			return
		}
		pending, ok := firstPending(elicitations)
		if !ok {
			steps = append(steps, "agent: no pending elicitation found")
			_ = json.NewEncoder(w).Encode(taskResponse{Steps: steps})
			return
		}
		if err := completeElicitation(stsURL, bearer, pending, req.Code); err != nil {
			steps = append(steps, "agent: completing elicitation failed: "+err.Error())
			_ = json.NewEncoder(w).Encode(taskResponse{Steps: steps})
			return
		}

		tools, toolServer, err := discoverTools(ctx, serverURLs, bearer)
		if err != nil {
			steps = append(steps, "agent: could not re-discover tools: "+err.Error())
			_ = json.NewEncoder(w).Encode(taskResponse{Steps: steps})
			return
		}
		if len(tools) == 0 {
			steps = append(steps, accessDeniedMessage)
			_ = json.NewEncoder(w).Encode(taskResponse{Steps: steps, Result: accessDeniedMessage})
			return
		}

		messages, call, server := loadPending()
		result := resumeLoop(ctx, llmURL, llmModel, bearer, stsURL, toolServer, tools, messages, call, server)
		steps = append(steps, result.Steps...)

		if result.ConsentURL != "" {
			savePending(result.Messages, result.PendingCall, result.PendingServer)
			_ = json.NewEncoder(w).Encode(taskResponse{Steps: steps, ConsentURL: result.ConsentURL})
			return
		}
		_ = json.NewEncoder(w).Encode(taskResponse{Steps: steps, Result: result.FinalText})
	})

	// Temporary diagnostic route: calls runbook-mcp's whoami tool directly,
	// bypassing the LLM loop entirely. gpt-4o was reliably declining to call
	// "whoami" itself (reads as a credential/identity probe regardless of
	// prompt phrasing) -- this lets us verify what the Keycloak-exchanged
	// token actually contains without depending on the model's cooperation.
	http.HandleFunc("/diagnostics/whoami", func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if _, err := verifyBearer(r, entraJWKSURL, entraIssuer, entraAudience); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		bearer := authHeader[len("Bearer "):]

		result, err := callToolJSON(r.Context(), runbookMCPURL, bearer, "whoami", nil)
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(result))
	})

	addr := envOr("ADDR", ":9200")
	log.Printf("agent listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
