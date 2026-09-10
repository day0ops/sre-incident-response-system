# CLAUDE.md

Guidance for Claude Code when working in this repo.

## Overview

On-call incident-response agent demo. Real MCP (Go SDK) over Enterprise Agentgateway, crossing from a Keycloak-federated-with-Entra domain into an Entra-native domain for the write path (deployment rollback), with one-time interactive consent (elicitation) for that crossing.

## Layout

- `agent/` - the agent itself (Go), LLM tool-calling loop against agentgateway-hosted OpenAI, MCP client, Entra token self-validation, elicitation handling
- `mcp-servers/{incident-mcp,runbook-mcp,repo-mcp}/` - real MCP servers (Go SDK), no server-side auth (agentgateway is the enforcement point)
- `chat-app/` - React/Vite frontend + Express backend (Entra login, session, proxies to the agent)
- `images/` - sequence diagram source (`sequence-diagram.mmd`) and generated assets

## Commands

```bash
go build ./... && go test ./...        # from repo root (go.work covers agent + all mcp-servers)
cd chat-app && npm run lint && npm test && npm run build
```

## Releases

Tagging `vX.Y.Z` and pushing the tag triggers CI to build/push all 5 images (agent, incident-mcp, runbook-mcp, repo-mcp, chat-app) and cut a GitHub release listing them. Check CI on `main` passed before tagging.

## Conventions

- No server-side JWT/role verification in the MCP servers or the agent's tool calls - trust the gateway entirely. The agent only self-validates its own token.
- Apache 2.0 license - keep it that way.
