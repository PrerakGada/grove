import { type ReactNode } from "react";

/*
  Grove marks every line it prints with one of four glyphs (internal/ui/ui.go):
  ✓ ok, → info, ! warn, ✗ error. Colouring on that glyph is the whole rule —
  the same rule the CLI itself uses — so terminal blocks on this page stay
  honest instead of being hand-highlighted for effect.
*/
const GLYPH: Record<string, string> = {
  "✓": "text-ok",
  "→": "text-info",
  "!": "text-warn",
  "✗": "text-err",
};

function Line({ text }: { text: string }) {
  const trimmed = text.trimStart();
  const indent = text.length - trimmed.length;
  const glyph = trimmed[0];
  const cls = GLYPH[glyph];

  if (text.startsWith("$ ")) {
    return (
      <div>
        <span className="select-none text-fg-dim">$ </span>
        <span className="text-fg">{text.slice(2)}</span>
      </div>
    );
  }

  if (!cls) {
    return <div className="text-fg-muted">{text || " "}</div>;
  }

  return (
    <div className="text-fg-muted">
      {" ".repeat(indent)}
      <span className={cls}>{glyph}</span>
      {trimmed.slice(1)}
    </div>
  );
}

export function Terminal({
  title,
  children,
  className = "",
}: {
  title?: string;
  children?: ReactNode;
  className?: string;
}) {
  return (
    <div
      className={`overflow-hidden rounded-xl border border-line bg-surface shadow-2xl shadow-black/60 ${className}`}
    >
      <div className="flex items-center gap-2 border-b border-line px-4 py-2.5">
        <span className="size-2.5 rounded-full bg-line-strong" />
        <span className="size-2.5 rounded-full bg-line-strong" />
        <span className="size-2.5 rounded-full bg-line-strong" />
        {title ? (
          <span className="ml-2 font-mono text-xs text-fg-dim">{title}</span>
        ) : null}
      </div>
      <div className="overflow-x-auto bg-term">{children}</div>
    </div>
  );
}

export function TerminalBody({ text }: { text: string }) {
  return (
    <pre className="min-w-max px-4 py-4 font-mono text-[13px] leading-[1.65]">
      {text.split("\n").map((l, i) => (
        <Line key={i} text={l} />
      ))}
    </pre>
  );
}
