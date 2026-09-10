import { AlertCircle, CheckCircle2, Loader2, ShieldCheck } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

export interface Turn {
  id: string;
  role: "user" | "assistant";
  text?: string;
  steps?: string[];
  result?: unknown;
  // Whether the response that produced `result` succeeded (HTTP-level, e.g. false
  // for the agent's 403 access-denied response) -- undefined for turns with no
  // backend response yet (e.g. the user's own message). Only affects the icon
  // shown next to `result`; it's not a general error state like `error` below.
  ok?: boolean;
  error?: string;
  consentUrl?: string;
}

export function ChatMessage({
  turn,
  busy,
  onApprove,
}: {
  turn: Turn;
  busy: boolean;
  onApprove: (consentUrl: string) => void;
}) {
  const isUser = turn.role === "user";
  const calledTools = (turn.steps ?? [])
    .map((step) => step.match(/^agent: calling (.+)$/)?.[1])
    .filter((name): name is string => Boolean(name));
  const hasContent = Boolean(
    turn.text ||
    (turn.result !== undefined && turn.result !== null) ||
    turn.error ||
    turn.consentUrl,
  );
  if (!hasContent) return null;

  return (
    <div className={cn("flex", isUser ? "justify-end" : "justify-start")}>
      <div
        className={cn(
          "max-w-[80%] rounded-xl px-3.5 py-2.5 text-sm ring-1",
          isUser
            ? "bg-primary text-primary-foreground ring-primary/10"
            : "bg-muted text-foreground ring-foreground/10",
        )}
      >
        {turn.text && <p className="whitespace-pre-wrap">{turn.text}</p>}

        {/* Step trace (tool calls + a duplicate copy of the final answer) hidden for
            now -- result below already shows the final answer on its own. */}

        {turn.result !== undefined && turn.result !== null && (
          <div className="mt-2">
            {turn.ok === false ? (
              <p className="flex items-center gap-1.5 text-destructive">
                <AlertCircle className="size-3.5 shrink-0" />
                Denied
              </p>
            ) : calledTools.length > 0 ? (
              <ul className="flex flex-col gap-1">
                {calledTools.map((tool, i) => (
                  <li
                    key={i}
                    className="flex items-center gap-1.5 text-green-600 dark:text-green-500"
                  >
                    <CheckCircle2 className="size-3.5 shrink-0" />
                    {tool}
                  </li>
                ))}
              </ul>
            ) : (
              <p className="flex items-center gap-1.5 text-green-600 dark:text-green-500">
                <CheckCircle2 className="size-3.5 shrink-0" />
                Done
              </p>
            )}
            <pre className="mt-1 overflow-x-auto rounded-lg bg-background/50 p-2 text-xs whitespace-pre-wrap">
              {typeof turn.result === "string" ? turn.result : JSON.stringify(turn.result, null, 2)}
            </pre>
          </div>
        )}

        {turn.error && (
          <p className="flex items-center gap-1.5 text-destructive">
            <AlertCircle className="size-3.5 shrink-0" />
            {turn.error}
          </p>
        )}

        {turn.consentUrl && (
          <Button
            variant="outline"
            size="sm"
            className="mt-2"
            disabled={busy}
            onClick={() => onApprove(turn.consentUrl!)}
          >
            {busy ? <Loader2 className="animate-spin" /> : <ShieldCheck />}
            {busy ? "Completing..." : "Approve cross-domain access"}
          </Button>
        )}
      </div>
    </div>
  );
}
