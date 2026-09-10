import { type ReactNode } from "react";
import { AlertCircle, CheckCircle2, Loader2, ShieldCheck } from "lucide-react";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";
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

// The LLM's own prose answer often comes back as markdown (bold labels, bullet
// lists, etc.) - render it properly instead of dumping literal "**text**"/
// "- item" syntax. Tailwind's preflight strips default list styling, so
// ul/ol need it restored explicitly or bullets just vanish.
function AssistantMarkdown({ text }: { text: string }) {
  return (
    <Markdown
      remarkPlugins={[remarkGfm]}
      components={{
        p: ({ children }: { children?: ReactNode }) => <p className="mb-2 last:mb-0">{children}</p>,
        ul: ({ children }: { children?: ReactNode }) => (
          <ul className="mb-2 list-disc space-y-0.5 pl-4 last:mb-0">{children}</ul>
        ),
        ol: ({ children }: { children?: ReactNode }) => (
          <ol className="mb-2 list-decimal space-y-0.5 pl-4 last:mb-0">{children}</ol>
        ),
        strong: ({ children }: { children?: ReactNode }) => (
          <strong className="font-semibold">{children}</strong>
        ),
        code: ({ children }: { children?: ReactNode }) => (
          <code className="rounded bg-background/70 px-1 py-0.5 font-mono text-xs break-all">
            {children}
          </code>
        ),
        a: ({ children, href }: { children?: ReactNode; href?: string }) => (
          <a href={href} target="_blank" rel="noreferrer" className="underline underline-offset-2">
            {children}
          </a>
        ),
        h1: ({ children }: { children?: ReactNode }) => (
          <p className="mb-1 font-semibold">{children}</p>
        ),
        h2: ({ children }: { children?: ReactNode }) => (
          <p className="mb-1 font-semibold">{children}</p>
        ),
        h3: ({ children }: { children?: ReactNode }) => (
          <p className="mb-1 font-semibold">{children}</p>
        ),
      }}
    >
      {text}
    </Markdown>
  );
}

// Structured results (objects, or a string that's actually JSON - e.g. the
// LLM echoing a tool's raw output verbatim) print as raw formatted JSON. Not
// a pretty table - that broke down under whoami's nested, long-valued claims
// across 3 servers. A plain <pre> just wraps normally with no column-width
// surprises.
function ResultView({ result }: { result: unknown }) {
  if (typeof result === "string") {
    const trimmed = result.trim();
    if (trimmed.startsWith("{") || trimmed.startsWith("[")) {
      try {
        const parsed = JSON.parse(trimmed);
        return (
          <pre className="mt-1 overflow-x-auto rounded-lg bg-background/50 p-2 text-xs whitespace-pre-wrap">
            {JSON.stringify(parsed, null, 2)}
          </pre>
        );
      } catch {
        // Not actually JSON - fall through to markdown below.
      }
    }
    return (
      <div className="mt-1 text-sm">
        <AssistantMarkdown text={result} />
      </div>
    );
  }

  return (
    <pre className="mt-1 overflow-x-auto rounded-lg bg-background/50 p-2 text-xs whitespace-pre-wrap">
      {JSON.stringify(result, null, 2)}
    </pre>
  );
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
          "max-w-[85%] rounded-xl px-3.5 py-2.5 text-sm ring-1",
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
            <ResultView result={turn.result} />
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
