// elicitation.go wraps Enterprise Agentgateway's own STS /elicitations
// lifecycle (GET list, PUT complete) -- the real, live-verified mechanism
// this repo's sibling day0ops/retail-returns-agent-system already proved out
// (see agentic-field-kit's docs/superpowers/plans/2026-08-31-retail-returns-phase9-agentgateway-elicitation.md
// and its own ui/server/elicitation.ts). A gated tool call doesn't carry the
// consent URL in its own failure response -- the caller has to separately ask
// the STS which elicitation is pending and what its OAuth details are.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type OAuthProviderInfo struct {
	ClientID     string   `json:"clientId"`
	AuthorizeURL string   `json:"authorizeUrl"`
	RedirectURI  string   `json:"redirectUri"`
	Scopes       []string `json:"scopes"`
}

type PendingElicitation struct {
	ID       int               `json:"id"`
	Resource string            `json:"resource"`
	Status   string            `json:"status"`
	OAuth    OAuthProviderInfo `json:"oauth"`
}

type rawElicitationEntry struct {
	Elicitation struct {
		ID       int    `json:"ID"`
		Resource string `json:"resource"`
		Status   string `json:"status"`
	} `json:"Elicitation"`
	OAuthConfig struct {
		ClientID     string   `json:"client_id"`
		AuthorizeURL string   `json:"authorize_url"`
		RedirectURI  string   `json:"redirect_uri"`
		Scopes       []string `json:"scopes"`
	} `json:"OAuthConfig"`
}

func listElicitations(stsURL, bearerToken string) ([]PendingElicitation, error) {
	req, err := http.NewRequest(http.MethodGet, stsURL+"/elicitations", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+bearerToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("elicitations list failed: HTTP %d", resp.StatusCode)
	}

	var entries []rawElicitationEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, err
	}
	out := make([]PendingElicitation, len(entries))
	for i, e := range entries {
		out[i] = PendingElicitation{
			ID:       e.Elicitation.ID,
			Resource: e.Elicitation.Resource,
			Status:   e.Elicitation.Status,
			OAuth: OAuthProviderInfo{
				ClientID:     e.OAuthConfig.ClientID,
				AuthorizeURL: e.OAuthConfig.AuthorizeURL,
				RedirectURI:  e.OAuthConfig.RedirectURI,
				Scopes:       e.OAuthConfig.Scopes,
			},
		}
	}
	return out, nil
}

// firstPending returns the first elicitation still awaiting completion, or
// ok=false if none are pending -- this demo only ever produces one at a time
// (the rollback_deployment gate), matching retail-returns' own "the caller
// just picks the pending one" simplification (its STS API has no per-resource
// filter either).
func firstPending(elicitations []PendingElicitation) (PendingElicitation, bool) {
	for _, e := range elicitations {
		if e.Status == "pending" {
			return e, true
		}
	}
	return PendingElicitation{}, false
}

// completeElicitation hands the STS a real OAuth authorization code; the STS
// performs the code -> token exchange itself and banks the resulting access
// token for the retried call to pick up.
func completeElicitation(stsURL, bearerToken string, e PendingElicitation, code string) error {
	body, err := json.Marshal(map[string]any{
		"id":       e.ID,
		"resource": e.Resource,
		"status":   "completed",
		"oauth_config": map[string]string{
			"code": code,
		},
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPut, stsURL+"/elicitations", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+bearerToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("elicitation completion failed: HTTP %d", resp.StatusCode)
	}
	return nil
}
