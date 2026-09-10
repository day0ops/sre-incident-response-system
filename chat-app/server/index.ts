// server/index.ts
import path from "node:path";
import { fileURLToPath } from "node:url";
import cookieParser from "cookie-parser";
import express from "express";
import { buildAuthorizeUrl, exchangeCodeForToken, type EntraChatConfig } from "./entra";

const staticDir = path.join(path.dirname(fileURLToPath(import.meta.url)), "..", "dist");

const cfg: EntraChatConfig = {
  authorizeUrl: process.env.ENTRA_AUTHORIZE_URL ?? "",
  tokenUrl: process.env.ENTRA_TOKEN_URL ?? "",
  clientId: process.env.ENTRA_CHAT_CLIENT_ID ?? "",
  clientSecret: process.env.ENTRA_CHAT_CLIENT_SECRET ?? "",
  redirectUri: process.env.ENTRA_CHAT_REDIRECT_URI ?? "http://localhost:5173/callback",
  scope: process.env.ENTRA_CHAT_SCOPE ?? "openid profile",
};
const agentUrl = process.env.AGENT_URL ?? "http://localhost:9200";

const app = express();
app.use(express.json());
app.use(cookieParser());

// In-memory session store keyed by a random id cookie -- fine for a local
// prototype, never for anything beyond it.
const sessions = new Map<string, string>();

app.get("/login", (_req, res) => {
  const state = Math.random().toString(36).slice(2);
  res.redirect(buildAuthorizeUrl(cfg, state));
});

app.get("/api/session", (req, res) => {
  const sessionId = req.cookies?.session;
  res.json({ loggedIn: Boolean(sessionId && sessions.has(sessionId)) });
});

app.get("/logout", (req, res) => {
  const sessionId = req.cookies?.session;
  if (sessionId) sessions.delete(sessionId);
  res.clearCookie("session");
  res.redirect("/");
});

app.get("/callback", async (req, res) => {
  const code = req.query.code as string;
  if (!code) {
    res.status(400).send("missing code");
    return;
  }
  try {
    const token = await exchangeCodeForToken(cfg, code);
    const sessionId = Math.random().toString(36).slice(2);
    sessions.set(sessionId, token);
    res.cookie("session", sessionId, { httpOnly: true });
    res.redirect("/");
  } catch (err) {
    res.status(500).send(String(err));
  }
});

app.post("/api/task", async (req, res) => {
  const sessionId = req.cookies?.session;
  const token = sessionId ? sessions.get(sessionId) : undefined;
  if (!token) {
    res.status(401).json({ error: "not logged in" });
    return;
  }
  try {
    const agentResp = await fetch(`${agentUrl}/task`, {
      method: "POST",
      headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
      body: JSON.stringify({ task: req.body.task }),
    });
    const body = await agentResp.json();
    res.status(agentResp.status).json(body);
  } catch (err) {
    res.status(502).json({ error: String(err) });
  }
});

// Temporary diagnostic route, mirrors /api/task's auth but hits the agent's
// non-LLM whoami passthrough -- see agent/main.go's /diagnostics/whoami.
app.get("/api/whoami", async (req, res) => {
  const sessionId = req.cookies?.session;
  const token = sessionId ? sessions.get(sessionId) : undefined;
  if (!token) {
    res.status(401).json({ error: "not logged in" });
    return;
  }
  try {
    const agentResp = await fetch(`${agentUrl}/diagnostics/whoami`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    const body = await agentResp.json();
    res.status(agentResp.status).json(body);
  } catch (err) {
    res.status(502).json({ error: String(err) });
  }
});

// The redirect target registered on the rollback_deployment elicitation's
// OAuth client (see agentic-field-kit's mcp-elicitation-policy usage for this
// usecase). Runs inside the consent popup the frontend opens, matching
// retail-returns-agent-system's Stage 9 popup/postMessage pattern -- hands the
// code back to the window that opened it, rather than navigating the main
// chat UI away.
app.get("/elicitation/callback", (req, res) => {
  const code = (req.query.code as string) ?? "";
  res.send(`<!doctype html><html><body><script>
    window.opener.postMessage({ type: "elicitation-code", code: ${JSON.stringify(code)} }, "*");
    window.close();
  </script></body></html>`);
});

app.post("/api/complete-rollback", async (req, res) => {
  const sessionId = req.cookies?.session;
  const token = sessionId ? sessions.get(sessionId) : undefined;
  if (!token) {
    res.status(401).json({ error: "not logged in" });
    return;
  }
  try {
    const agentResp = await fetch(`${agentUrl}/task/complete-rollback`, {
      method: "POST",
      headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
      body: JSON.stringify({ code: req.body.code }),
    });
    const body = await agentResp.json();
    res.status(agentResp.status).json(body);
  } catch (err) {
    res.status(502).json({ error: String(err) });
  }
});

// Serves the Vite-built static assets in production (a single process, matching
// this repo's other components); in dev mode Vite's own dev server on :5173
// handles the frontend and proxies /api, /login, /callback here instead.
app.use(express.static(staticDir));
app.get("/", (_req, res) => res.sendFile(path.join(staticDir, "index.html")));

const port = Number(process.env.PORT ?? 5174);
app.listen(port, () => console.log(`chat-app server listening on :${port}`));
