// src/App.tsx
import { useEffect, useRef, useState, type KeyboardEvent } from "react";
import { CheckCircle2, Fingerprint, Loader2, LogIn, LogOut, Send, Workflow } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { TooltipProvider } from "@/components/ui/tooltip";
import { ChatMessage, type Turn } from "@/components/chat-message";
import { TypingIndicator } from "@/components/typing-indicator";
import { ThemeToggle } from "@/components/theme-toggle";
import { SequenceDiagram } from "@/components/sequence-diagram";
import { SEQUENCE_PARTICIPANTS, SEQUENCE_SECTIONS } from "@/lib/sequence-data";

interface TaskResponse {
  steps?: string[];
  result?: unknown;
  consent_url?: string;
  error?: string;
}

let nextId = 0;
const newId = () => `turn-${++nextId}`;

function responseToTurn(response: TaskResponse, ok: boolean): Turn {
  return {
    id: newId(),
    role: "assistant",
    steps: response.steps,
    result: response.result,
    ok,
    error: response.steps ? undefined : (response.error ?? "Please log in first"),
    consentUrl: response.consent_url,
  };
}

export function App() {
  const [task, setTask] = useState("");
  const [messages, setMessages] = useState<Turn[]>([]);
  const [busy, setBusy] = useState(false);
  const [loggedIn, setLoggedIn] = useState(false);
  const [version, setVersion] = useState<string | null>(null);
  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages, busy]);

  useEffect(() => {
    fetch("/api/session")
      .then((res) => res.json())
      .then((body: { loggedIn?: boolean }) => setLoggedIn(Boolean(body.loggedIn)));
  }, []);

  useEffect(() => {
    fetch("/api/version")
      .then((res) => res.json())
      .then((body: { version?: string }) => setVersion(body.version ?? null))
      .catch(() => {});
  }, []);

  async function submit() {
    const text = task.trim();
    if (!text || busy) return;

    setMessages((prev) => [...prev, { id: newId(), role: "user", text }]);
    setTask("");
    setBusy(true);
    try {
      const res = await fetch("/api/task", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ task: text }),
      });
      const body: TaskResponse = await res.json();
      setMessages((prev) => [...prev, responseToTurn(body, res.ok)]);
    } catch (err) {
      setMessages((prev) => [
        ...prev,
        { id: newId(), role: "assistant", error: `Request failed: ${String(err)}` },
      ]);
    } finally {
      setBusy(false);
    }
  }

  function approve(consentUrl: string) {
    window.open(consentUrl, "elicitation-consent", "width=500,height=700");
  }

  // Bypasses the LLM loop entirely (see agent/main.go's /diagnostics/whoami) -
  // the model reliably declines to call a tool named "whoami" itself, so this
  // is the only reliable way to show the token each of the 3 MCP servers
  // actually received.
  async function checkIdentity() {
    if (busy) return;
    setMessages((prev) => [
      ...prev,
      { id: newId(), role: "user", text: "Check whoami on all servers" },
    ]);
    setBusy(true);
    try {
      const res = await fetch("/api/whoami");
      const body = await res.json();
      setMessages((prev) => [
        ...prev,
        { id: newId(), role: "assistant", result: body, ok: res.ok },
      ]);
    } catch (err) {
      setMessages((prev) => [
        ...prev,
        { id: newId(), role: "assistant", error: `Request failed: ${String(err)}` },
      ]);
    } finally {
      setBusy(false);
    }
  }

  useEffect(() => {
    function onMessage(event: MessageEvent) {
      if (event.data?.type !== "elicitation-code") return;
      setBusy(true);
      fetch("/api/complete-rollback", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ code: event.data.code }),
      })
        .then((res) => res.json().then((body: TaskResponse) => responseToTurn(body, res.ok)))
        .then((turn) => setMessages((prev) => [...prev, turn]))
        .catch((err) =>
          setMessages((prev) => [
            ...prev,
            { id: newId(), role: "assistant", error: `Request failed: ${String(err)}` },
          ]),
        )
        .finally(() => setBusy(false));
    }
    window.addEventListener("message", onMessage);
    return () => window.removeEventListener("message", onMessage);
  }, []);

  function onKeyDown(e: KeyboardEvent<HTMLTextAreaElement>) {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      submit();
    }
  }

  return (
    <TooltipProvider>
      <div className="flex min-h-screen items-center justify-center bg-background p-6">
        <div className="flex h-[80vh] w-full max-w-2xl flex-col overflow-hidden rounded-xl bg-card ring-1 ring-foreground/10">
          <header className="flex items-center justify-between border-b border-border px-6 py-4">
            <div>
              <p className="text-xs font-semibold tracking-widest text-muted-foreground uppercase">
                SRE Incident Response System
              </p>
              <h1 className="font-heading text-lg font-medium">Incident copilot</h1>
              {version && <p className="text-[11px] text-muted-foreground">{version}</p>}
            </div>
            <div className="flex items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                aria-label="Check identity across all servers"
                disabled={busy}
                onClick={checkIdentity}
              >
                <Fingerprint />
              </Button>
              <Dialog>
                <DialogTrigger asChild>
                  <Button variant="outline" size="sm" aria-label="View sequence diagram">
                    <Workflow />
                  </Button>
                </DialogTrigger>
                <DialogContent className="max-w-[min(92vw,1200px)]">
                  <DialogTitle>Cross-domain MCP identity federation</DialogTitle>
                  <DialogDescription>
                    Hover or focus a step for the full mechanics behind each hop.
                  </DialogDescription>
                  <SequenceDiagram
                    participants={SEQUENCE_PARTICIPANTS}
                    sections={SEQUENCE_SECTIONS}
                  />
                </DialogContent>
              </Dialog>
              <ThemeToggle />
              {loggedIn ? (
                <>
                  <Badge variant="success">
                    <CheckCircle2 />
                    Logged in
                  </Badge>
                  <Button variant="ghost" size="sm" asChild>
                    <a href="/logout">
                      <LogOut />
                      Log out
                    </a>
                  </Button>
                </>
              ) : (
                <Button variant="outline" size="sm" asChild>
                  <a href="/login">
                    <LogIn />
                    Log in with Entra
                  </a>
                </Button>
              )}
            </div>
          </header>

          <div className="scrollbar-thin flex flex-1 flex-col gap-3 overflow-y-auto px-6 py-4">
            {messages.length === 0 && !busy && (
              <p className="text-sm text-muted-foreground">
                Describe an incident to start investigating, e.g. "checkout is returning 500s".
              </p>
            )}
            {messages.map((turn) => (
              <ChatMessage key={turn.id} turn={turn} busy={busy} onApprove={approve} />
            ))}
            {busy && (
              <div className="flex justify-start">
                <TypingIndicator />
              </div>
            )}
            <div ref={bottomRef} />
          </div>

          <div className="border-t border-border px-6 py-4">
            <div className="flex items-stretch gap-2">
              <textarea
                value={task}
                onChange={(e) => setTask(e.target.value)}
                onKeyDown={onKeyDown}
                placeholder="Describe the incident..."
                rows={2}
                className="flex-1 resize-none rounded-lg border border-input bg-background px-3 py-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
              />
              <Button
                onClick={submit}
                disabled={busy || !task.trim()}
                size="lg"
                className="h-auto px-4"
              >
                {busy ? <Loader2 className="animate-spin" /> : <Send />}
                Investigate
              </Button>
            </div>
          </div>
        </div>
      </div>
    </TooltipProvider>
  );
}
