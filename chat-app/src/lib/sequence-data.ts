import type { SequenceSection } from "@/components/sequence-diagram";

// Distilled from images/sequence-diagram.mmd - the diagram itself stays
// minimal (short labels only); the fuller mechanics live in each step's
// `detail`, shown on hover/focus. Each section gets its own color and title
// (rendered as a divider row), matching the mermaid file's `Note over` blocks.
export const PHASE_COLORS = {
  login: "#10b981",
  readA: "#3b82f6",
  readB: "#8b5cf6",
  write: "#f59e0b",
  denied: "#ef4444",
} as const;

export const SEQUENCE_PARTICIPANTS = [
  "SRE",
  "Chat App",
  "Agent",
  "Gateway",
  "Keycloak",
  "Entra",
  "incident-mcp",
  "runbook-mcp",
  "repo-mcp",
];

export const SEQUENCE_SECTIONS: SequenceSection[] = [
  {
    title: "Login (once per session)",
    color: PHASE_COLORS.login,
    steps: [
      { from: "SRE", to: "Chat App", label: "open app" },
      {
        from: "Chat App",
        to: "Entra",
        label: "OIDC redirect",
        detail:
          "Chat app redirects to Entra's authorization endpoint (OIDC Authorization Code flow).",
      },
      { from: "Entra", to: "SRE", label: "login prompt" },
      { from: "SRE", to: "Entra", label: "credentials" },
      {
        from: "Entra",
        to: "Chat App",
        label: "Entra JWT",
        detail: "Entra issues a JWT audienced to agent-gateway (aud=agent-gateway).",
      },
      { from: "Chat App", to: "Agent", label: "task + JWT" },
    ],
  },
  {
    title: "Read path - Keycloak-federated home domain, silent",
    color: PHASE_COLORS.readA,
    steps: [
      {
        from: "Agent",
        to: "Agent",
        label: "validate JWT",
        self: true,
        detail:
          "The agent validates its own Entra JWT before acting - a self-check, not server-side enforcement.",
      },
      {
        from: "Agent",
        to: "Gateway",
        label: "get_unhealthy_pods",
        detail: "MCP tools/call, Bearer Entra JWT.",
      },
      {
        from: "Gateway",
        to: "Gateway",
        label: "validate JWT",
        self: true,
        detail: "JWT auth policy validates the Entra JWT against Entra's external JWKS.",
      },
      {
        from: "Gateway",
        to: "Keycloak",
        label: "token exchange",
        detail: "RFC 8693 token exchange: subject_token = the caller's Entra JWT.",
      },
      {
        from: "Keycloak",
        to: "Keycloak",
        label: "validate via broker",
        self: true,
        detail:
          "Keycloak validates the incoming Entra JWT via its Entra identity-provider (broker) federation.",
      },
      {
        from: "Keycloak",
        to: "Gateway",
        label: "Keycloak JWT",
        detail: "Downscoped, Keycloak-issued JWT audienced to incident-mcp.",
      },
      {
        from: "Gateway",
        to: "incident-mcp",
        label: "get_unhealthy_pods",
        detail:
          "Bearer Keycloak JWT. incident-mcp performs no JWT verification of its own - trusts agentgateway entirely.",
      },
      {
        from: "incident-mcp",
        to: "Gateway",
        label: "3 unhealthy pods",
        detail: "Postgres connection failing.",
      },
      { from: "Gateway", to: "Agent", label: "result" },
    ],
  },
  {
    title: "Read path - same exchange, plus a role-gated tool",
    color: PHASE_COLORS.readB,
    steps: [
      { from: "Agent", to: "Gateway", label: "get_deployment_history" },
      {
        from: "Gateway",
        to: "Keycloak",
        label: "token exchange",
        detail: "Downscopes to the caller's Entra role claim (e.g. deployment.history.read).",
      },
      {
        from: "Gateway",
        to: "Gateway",
        label: "role check (CEL)",
        self: true,
        detail:
          "mcp-tool-policy requires 'deployment.history.read' in jwt.roles - checked against the caller's own Entra JWT, not the downscoped Keycloak token.",
      },
      {
        from: "Gateway",
        to: "runbook-mcp",
        label: "get_deployment_history",
        detail: "Only reached if the role check passes; denied requests never reach runbook-mcp.",
      },
      { from: "runbook-mcp", to: "Gateway", label: "v42 (8m ago), prev v41" },
      { from: "Gateway", to: "Agent", label: "result" },
    ],
  },
  {
    // This diagram is linear, not branching - the mermaid's alt/else is shown as
    // its own mini-sequence rather than omitted, since the denial path is as much
    // a part of the demo as the success path above it.
    title: "Alternate outcome - denied by the role check above",
    color: PHASE_COLORS.denied,
    steps: [
      {
        from: "Gateway",
        to: "Agent",
        label: "denied (role missing)",
        detail:
          "Alternate outcome of the role check above: if the caller's Entra JWT lacks 'deployment.history.read', mcp-tool-policy denies the call before it ever reaches runbook-mcp.",
      },
      { from: "Agent", to: "Chat App", label: "access-denied error" },
    ],
  },
  {
    title: "Write path - Entra-native domain, never federated with Keycloak",
    color: PHASE_COLORS.write,
    steps: [
      {
        from: "Agent",
        to: "Gateway",
        label: "rollback_deployment",
        detail:
          "repo-mcp sits behind its own dedicated HTTPRoute, not covered by the shared JWT-auth policy.",
      },
      {
        from: "Gateway",
        to: "Entra",
        label: "On-Behalf-Of exchange",
        detail:
          "grant_type=jwt-bearer, scope=api://repo-mcp/.default. Requires admin consent granted in advance - silent, no interactive popup, no banked token.",
      },
      { from: "Entra", to: "Gateway", label: "repo-mcp token" },
      {
        from: "Gateway",
        to: "repo-mcp",
        label: "rollback_deployment",
        detail:
          "Bearer Entra OBO token. No JWT/role check here - unlike get_deployment_history, gated only by Entra's admin-consent/assignment.",
      },
      { from: "repo-mcp", to: "Gateway", label: "rollback successful, 5/5 healthy" },
      { from: "Gateway", to: "Agent", label: "result" },
      { from: "Agent", to: "Chat App", label: "confirmation" },
    ],
  },
];
