// Standalone mock backend for manually testing chat-app in a real browser,
// without needing real Entra credentials or a running agent/MCP stack.
// Implements just the routes Vite's dev proxy forwards from :5173 to :5174.
// Not part of the app - temporary local testing aid, gitignored.
import express from "express";
import cookieParser from "cookie-parser";

const app = express();
app.use(express.json());
app.use(cookieParser());

app.get("/api/session", (_req, res) => res.json({ loggedIn: true }));
app.get("/login", (_req, res) => res.redirect("/"));
app.get("/logout", (_req, res) => res.redirect("/"));

let callCount = 0;

app.post("/api/task", (req, res) => {
  const task = String(req.body?.task ?? "").toLowerCase();
  callCount += 1;

  if (task.includes("roll") || task.includes("rollback")) {
    res.json({
      steps: [
        "agent: calling get_unhealthy_pods",
        "agent: calling get_deployment_history",
        "agent: rollback_deployment requires one-time consent (elicitation)",
      ],
      consent_url: "http://localhost:5174/elicitation/callback?code=mock-code-123",
    });
    return;
  }

  if (task.includes("deploy") || task.includes("history")) {
    // Alternate success/denial each call, so both paths are easy to click through.
    if (callCount % 2 === 0) {
      res.json({
        steps: ["agent: calling get_deployment_history"],
        result: "checkout-api v42 deployed 8 minutes ago, previous version v41",
      });
    } else {
      res.status(403).json({
        steps: ["agent: calling get_deployment_history"],
        error: "denied: 'deployment.history.read' not present in your Entra roles",
      });
    }
    return;
  }

  res.json({
    steps: ["agent: calling get_unhealthy_pods", "agent: calling get_deployment_history"],
    result:
      "3 pods CrashLoopBackOff in checkout namespace, postgres connection failures. checkout-api v42 deployed 8 minutes ago, previous known-good version v41.",
  });
});

app.post("/api/complete-rollback", (_req, res) => {
  res.json({
    steps: ["agent: completing rollback consent", "agent: calling rollback_deployment"],
    result: "checkout-api rolled back to v41, 5/5 replicas healthy",
  });
});

app.get("/elicitation/callback", (req, res) => {
  const code = req.query.code ?? "";
  res.send(`<!doctype html><html><body><script>
    window.opener.postMessage({ type: "elicitation-code", code: ${JSON.stringify(code)} }, "*");
    window.close();
  </script></body></html>`);
});

app.listen(5174, () => console.log("mock chat-app backend listening on :5174"));
