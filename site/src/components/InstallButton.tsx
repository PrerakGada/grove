"use client";

import { useEffect, useRef, useState } from "react";
import { CopyCommand } from "./CopyCommand";

/*
  The install command is not printed on the page. This button carries
  `data-pt-command`, so the shared product counter (tracker.js from
  api.prerakgada.in, tagged in layout.tsx) takes the click, asks the optional
  "who is installing" form, and shows the command after it — submitted or
  skipped. The command itself reaches the tracker through the script tag's
  `data-command`.

  An install must never depend on the counter. If the tracker never loaded (an
  ad blocker, a network error) or did not take the click (its stylesheet had
  not arrived yet), the command is revealed right here instead, in the same
  copy-to-clipboard block the page used before. JavaScript off is covered by
  the <noscript> copies in page.tsx.
*/
export function InstallButton({
  command,
  className = "",
}: {
  command: string;
  className?: string;
}) {
  const [revealed, setRevealed] = useState(false);
  const box = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (revealed) box.current?.querySelector("button")?.focus();
  }, [revealed]);

  if (revealed) {
    return (
      <div ref={box} className={`min-w-0 ${className}`}>
        <CopyCommand command={command} className="w-full" />
      </div>
    );
  }

  return (
    <button
      type="button"
      data-pt-command=""
      onClick={(event) => {
        const click = event.nativeEvent;
        // React and the tracker both listen on document, in no fixed order.
        // Look once every listener has run: the tracker either cancelled the
        // click or opened its dialog synchronously.
        window.setTimeout(() => {
          if (!trackerTookClick(click)) setRevealed(true);
        }, 250);
      }}
      className={`flex items-center justify-center gap-2.5 rounded-lg border border-ok/40 bg-ok/10 px-4 py-3 text-sm font-medium text-fg transition-colors hover:border-ok/70 hover:bg-ok/15 ${className}`}
    >
      <span className="font-mono text-ok" aria-hidden="true">
        $
      </span>
      Install with Go
    </button>
  );
}

function trackerTookClick(click: Event): boolean {
  if (click.defaultPrevented) return true;
  const host = document.querySelector("[data-product-tracker]");
  return Boolean(host?.shadowRoot?.querySelector("dialog")?.open);
}
