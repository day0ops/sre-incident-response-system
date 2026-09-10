import { useEffect, useRef } from "react";
import { motion } from "framer-motion";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";

export interface SequenceStep {
  from: string;
  to: string;
  label: string;
  detail?: string;
  self?: boolean;
}

export interface SequenceSection {
  title: string;
  color: string;
  steps: SequenceStep[];
}

interface SequenceDiagramProps {
  participants: string[];
  sections: SequenceSection[];
}

type Row =
  | { kind: "divider"; title: string; color: string }
  | { kind: "step"; step: SequenceStep; color: string };

/**
 * A from-scratch animated sequence diagram (not a diagramming library):
 * vertical lifelines, one per participant, with each step's arrow fading in
 * in order down the page so the actual temporal flow of a request reads
 * clearly. Sections get their own divider row (title + color), matching the
 * source mermaid file's `Note over` blocks, so the login/read/write phases
 * read as distinct stages rather than one long undifferentiated list.
 *
 * The participant header lives in its own small sticky element, a sibling of
 * the scrolling body (not nested inside it) - `overflow-x-auto` on the body
 * implicitly upgrades its own `overflow-y` to `auto` too (a CSS quirk: one
 * non-`visible` axis forces the other off `visible`), which would hijack
 * `position: sticky` before it ever reaches the dialog's real scroll
 * container if the header were nested inside. Horizontal scroll is instead
 * synced from body to header with a plain scroll listener.
 *
 * Kept deliberately minimal (short on-diagram labels only) - hover or focus
 * a step for the fuller mechanics via tooltip.
 */
export function SequenceDiagram({ participants, sections }: SequenceDiagramProps) {
  const headerRef = useRef<HTMLDivElement>(null);
  const bodyRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const header = headerRef.current;
    const body = bodyRef.current;
    if (!header || !body) return;
    const syncHeader = () => {
      header.scrollLeft = body.scrollLeft;
    };
    body.addEventListener("scroll", syncHeader, { passive: true });
    return () => body.removeEventListener("scroll", syncHeader);
  }, []);

  const rows: Row[] = sections.flatMap((section) => [
    { kind: "divider" as const, title: section.title, color: section.color },
    ...section.steps.map((step) => ({ kind: "step" as const, step, color: section.color })),
  ]);

  // Deliberately tight - wide enough for labels, but with real margin against
  // the dialog's max width so a horizontal scrollbar doesn't reappear on
  // anything but genuinely small windows.
  const colWidth = 108;
  const width = Math.max(560, participants.length * colWidth);
  const rowHeight = 40;
  // Tall enough that a divider's title pill never crowds the very next
  // arrow's label above it - the gap has to clear the label's actual glyph
  // height (~8px above its baseline), not just baseline-to-baseline spacing,
  // and self-loop labels sit even closer to their line than plain arrows do.
  const dividerHeight = 70;
  const topPad = 16;
  const headerHeight = 32;
  const height =
    topPad +
    rows.reduce((sum, row) => sum + (row.kind === "divider" ? dividerHeight : rowHeight), 0) +
    16;
  const colX = (name: string) => {
    const i = participants.indexOf(name);
    return participants.length === 1
      ? width / 2
      : 48 + (i / (participants.length - 1)) * (width - 96);
  };

  let y = topPad;
  const positioned = rows.map((row) => {
    const rowY = y;
    y += row.kind === "divider" ? dividerHeight : rowHeight;
    return { row, y: rowY };
  });

  return (
    <div className="w-full min-w-0 rounded-xl border border-border bg-card">
      <div
        ref={headerRef}
        className="sticky top-0 z-10 overflow-x-hidden rounded-t-xl border-b border-border bg-card/35 px-4 py-1.5 backdrop-blur-sm"
      >
        <svg viewBox={`0 0 ${width} ${headerHeight}`} style={{ minWidth: width }} aria-hidden>
          {participants.map((p) => (
            <text
              key={p}
              x={colX(p)}
              y={headerHeight / 2 + 4}
              textAnchor="middle"
              fontSize={11}
              fontWeight={600}
              fill="var(--foreground)"
            >
              {p}
            </text>
          ))}
        </svg>
      </div>

      <div ref={bodyRef} className="scrollbar-thin overflow-x-auto rounded-b-xl p-4 pt-2">
        <svg
          viewBox={`0 0 ${width} ${height}`}
          style={{ minWidth: width }}
          role="img"
          aria-label="Sequence diagram of the cross-domain MCP identity federation flow"
        >
          <defs>
            {/* fill="context-stroke" inherits the referencing line/path's own stroke
                color, so one marker def serves every phase color without duplication. */}
            <marker id="seq-arrow" markerWidth="8" markerHeight="8" refX="6" refY="4" orient="auto">
              <path d="M0,0 L8,4 L0,8 Z" fill="context-stroke" />
            </marker>
          </defs>

          {participants.map((p) => (
            <line
              key={p}
              x1={colX(p)}
              y1={0}
              x2={colX(p)}
              y2={height - 8}
              stroke="var(--border)"
              strokeDasharray="3 3"
            />
          ))}

          {positioned.map(({ row, y: rowY }, i) => {
            if (row.kind === "divider") {
              // Sized to the actual title (titles range from ~25 to ~65 characters) -
              // a fixed pill width would either clip long titles or look oversized
              // next to short ones.
              const pillWidth = Math.min(width - 32, Math.max(160, row.title.length * 5.6 + 28));
              return (
                <motion.g
                  key={i}
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  transition={{ delay: i * 0.12, duration: 0.25 }}
                >
                  <line
                    x1={8}
                    y1={rowY + dividerHeight / 2}
                    x2={width - 8}
                    y2={rowY + dividerHeight / 2}
                    stroke={row.color}
                    strokeOpacity={0.35}
                  />
                  <rect
                    x={width / 2 - pillWidth / 2}
                    y={rowY + dividerHeight / 2 - 11}
                    width={pillWidth}
                    height={22}
                    rx={11}
                    fill="var(--card)"
                    stroke={row.color}
                    strokeOpacity={0.5}
                  />
                  <text
                    x={width / 2}
                    y={rowY + dividerHeight / 2 + 4}
                    textAnchor="middle"
                    fontSize={10.5}
                    fontWeight={600}
                    fill={row.color}
                  >
                    {row.title}
                  </text>
                </motion.g>
              );
            }

            const { step, color } = row;
            const x1 = colX(step.from);
            const x2 = step.self ? x1 + 24 : colX(step.to);
            const hitX = Math.min(x1, x2) - 12;
            const hitWidth = Math.abs(x2 - x1) + 24;

            const arrow = step.self ? (
              <>
                <text x={x1 + 14} y={rowY - 4} fontSize={10} fill="var(--muted-foreground)">
                  {step.label}
                </text>
                <path
                  d={`M ${x1} ${rowY - 8} q 24 8 0 16`}
                  fill="none"
                  stroke={color}
                  strokeWidth={1.5}
                  markerEnd="url(#seq-arrow)"
                />
              </>
            ) : (
              <>
                <text
                  x={(x1 + x2) / 2}
                  y={rowY - 6}
                  textAnchor="middle"
                  fontSize={10}
                  fill="var(--muted-foreground)"
                >
                  {step.label}
                </text>
                <line
                  x1={x1}
                  y1={rowY}
                  x2={x2}
                  y2={rowY}
                  stroke={color}
                  strokeWidth={1.5}
                  markerEnd="url(#seq-arrow)"
                />
              </>
            );

            const group = (
              <motion.g
                key={i}
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                transition={{ delay: i * 0.12, duration: 0.25 }}
                tabIndex={step.detail ? 0 : undefined}
                className={step.detail ? "cursor-help outline-none" : undefined}
              >
                {/* Wider transparent hit area so hovering/focusing near the thin
                    arrow (not just directly on the 1px line) still triggers the tooltip. */}
                <rect x={hitX} y={rowY - 18} width={hitWidth} height={26} fill="transparent" />
                {arrow}
              </motion.g>
            );

            if (!step.detail) return group;

            return (
              <Tooltip key={i}>
                <TooltipTrigger asChild>{group}</TooltipTrigger>
                <TooltipContent side="bottom" className="max-w-sm text-left whitespace-normal">
                  {step.detail}
                </TooltipContent>
              </Tooltip>
            );
          })}
        </svg>
      </div>
    </div>
  );
}
