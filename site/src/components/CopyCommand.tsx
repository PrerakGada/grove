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
      className={`group flex items-start gap-3 rounded-lg border border-line bg-surface px-4 py-3 text-left font-mono text-[12px] transition-colors hover:border-line-strong sm:items-center sm:text-[13px] ${className}`}
    >
      <span className="select-none text-fg-dim">$</span>
      {/* wraps rather than truncating — this is the primary CTA and the whole
          command has to be readable on a phone, not just copyable */}
      <span className="min-w-0 flex-1 break-all text-fg">{command}</span>
      <span
        className={`shrink-0 text-xs transition-colors ${
          copied ? "text-ok" : "text-fg-dim group-hover:text-fg-muted"
        }`}
        aria-hidden="true"
      >
        {copied ? "copied" : "copy"}
      </span>
      {/* the visible label sits inside the button, so aria-label hides it from
          assistive tech — this always-mounted live region is what actually
          announces the copy */}
      <span role="status" className="sr-only">
        {copied ? "Copied to clipboard" : ""}
      </span>
    </button>
  );
}
