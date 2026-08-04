"use client";

import { useRef, useState } from "react";
import { Terminal, TerminalBody } from "./Terminal";
import { lifecycle } from "@/lib/content";

/*
  A real ARIA tabs implementation, not just the role names: roving tabindex,
  arrow/Home/End keys, and a labelled tabpanel. Declaring role="tablist"
  without those is worse for a screen-reader user than plain buttons.
*/
export function Lifecycle() {
  const [i, setI] = useState(1);
  const tabs = useRef<(HTMLButtonElement | null)[]>([]);
  const step = lifecycle[i];

  function onKeyDown(e: React.KeyboardEvent) {
    const last = lifecycle.length - 1;
    let next: number | null = null;

    if (e.key === "ArrowRight" || e.key === "ArrowDown") next = i === last ? 0 : i + 1;
    else if (e.key === "ArrowLeft" || e.key === "ArrowUp") next = i === 0 ? last : i - 1;
    else if (e.key === "Home") next = 0;
    else if (e.key === "End") next = last;

    if (next === null) return;
    e.preventDefault();
    setI(next);
    tabs.current[next]?.focus();
  }

  return (
    <div className="grid gap-6 lg:grid-cols-[15rem_1fr] lg:gap-10">
      <div
        className="flex gap-2 overflow-x-auto pb-2 lg:flex-col lg:gap-1 lg:overflow-visible lg:pb-0"
        role="tablist"
        aria-label="Grove lifecycle"
        aria-orientation="vertical"
        onKeyDown={onKeyDown}
      >
        {lifecycle.map((s, n) => {
          const on = n === i;
          return (
            <button
              key={s.cmd}
              ref={(el) => {
                tabs.current[n] = el;
              }}
              id={`lc-tab-${n}`}
              role="tab"
              aria-selected={on}
              aria-controls="lc-panel"
              tabIndex={on ? 0 : -1}
              onClick={() => setI(n)}
              className={`shrink-0 rounded-lg border px-3.5 py-2.5 text-left transition-colors lg:shrink ${
                on
                  ? "border-line-strong bg-surface"
                  : "border-transparent hover:bg-surface/60"
              }`}
            >
              <span
                className={`block font-mono text-[13px] ${on ? "text-ok" : "text-fg-muted"}`}
              >
                {s.cmd}
              </span>
              <span className="mt-0.5 hidden text-xs text-fg-dim lg:block">
                {s.label}
              </span>
            </button>
          );
        })}
      </div>

      <div
        id="lc-panel"
        role="tabpanel"
        aria-labelledby={`lc-tab-${i}`}
        tabIndex={0}
        className="min-w-0 focus-visible:outline-2"
      >
        <p className="mb-4 max-w-2xl text-[15px] leading-relaxed text-fg-muted">
          {step.blurb}
        </p>
        <Terminal title={step.cmd}>
          <TerminalBody text={step.term} />
        </Terminal>
      </div>
    </div>
  );
}
