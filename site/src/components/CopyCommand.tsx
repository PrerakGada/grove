"use client";

import { useState } from "react";

export function CopyCommand({
  command,
  className = "",
}: {
  command: string;
  className?: string;
}) {
  const [copied, setCopied] = useState(false);

  return (
    <button
      type="button"
      onClick={() => {
        navigator.clipboard.writeText(command).then(() => {
          setCopied(true);
          setTimeout(() => setCopied(false), 1600);
        });
      }}
      aria-label={`Copy: ${command}`}
      className={`group flex items-center gap-3 rounded-lg border border-line bg-surface px-4 py-3 text-left font-mono text-[13px] transition-colors hover:border-line-strong ${className}`}
    >
      <span className="select-none text-fg-dim">$</span>
      <span className="min-w-0 flex-1 truncate text-fg">{command}</span>
      <span
        className={`shrink-0 text-xs transition-colors ${
          copied ? "text-ok" : "text-fg-dim group-hover:text-fg-muted"
        }`}
      >
        {copied ? "copied" : "copy"}
      </span>
    </button>
  );
}
