// Shown in place of the next assistant turn while a request is in flight -
// there's no streaming/step data from the backend (a single blocking await),
// so this is a generic "the agent is working" cue rather than real progress.
export function TypingIndicator() {
  return (
    <div className="flex w-fit items-center gap-1 rounded-xl bg-muted px-3.5 py-3 ring-1 ring-foreground/10">
      <span className="size-1.5 animate-bounce rounded-full bg-muted-foreground [animation-delay:-0.3s]" />
      <span className="size-1.5 animate-bounce rounded-full bg-muted-foreground [animation-delay:-0.15s]" />
      <span className="size-1.5 animate-bounce rounded-full bg-muted-foreground" />
    </div>
  );
}
