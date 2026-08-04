"use client";

import { useRef, useState } from "react";

/*
  Only the tab state is client-side. The panels arrive as already-rendered
  server children, so the captured terminal output never enters the client
  bundle — which is also why this component imports nothing from content.ts.

  A real ARIA tabs implementation, not just the role names: roving tabindex,
  arrow/Home/End keys, and one labelled tabpanel per tab. Every panel is in the
  markup and inactive ones are `hidden`, so a crawler or a reader without JS
  gets all five captures rather than only the selected one.
*/

export type LifecycleTab = { cmd: string; label: string };

export function Lifecycle({
  tabs,
  panels,
}: {
  tabs: LifecycleTab[];
  panels: React.ReactNode[];
}) {
  const [i, setI] = useState(1);
  const refs = useRef<(HTMLButtonElement | null)[]>([]);

  function onKeyDown(e: React.KeyboardEvent) {
    const last = tabs.length - 1;
    let next: number | null = null;

    if (e.key === "ArrowRight" || e.key === "ArrowDown")
      next = i === last ? 0 : i + 1;
    else if (e.key === "ArrowLeft" || e.key === "ArrowUp")
      next = i === 0 ? last : i - 1;
    else if (e.key === "Home") next = 0;
    else if (e.key === "End") next = last;

    if (next === null) return;
    e.preventDefault();
    setI(next);
    refs.current[next]?.focus();
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
        {tabs.map((t, n) => {
          const on = n === i;
          return (
            <button
              key={t.cmd}
              ref={(el) => {
                refs.current[n] = el;
              }}
              id={`lc-tab-${n}`}
              role="tab"
              aria-selected={on}
              aria-controls={`lc-panel-${n}`}
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
                {t.cmd}
              </span>
              <span className="mt-0.5 hidden text-xs text-fg-dim lg:block">
                {t.label}
              </span>
            </button>
          );
        })}
      </div>

      <div className="min-w-0">
        {panels.map((p, n) => (
          <div
            key={n}
            id={`lc-panel-${n}`}
            role="tabpanel"
            aria-labelledby={`lc-tab-${n}`}
            hidden={n !== i}
            tabIndex={0}
            className="min-w-0 focus-visible:outline-2"
          >
            {p}
          </div>
        ))}
      </div>
    </div>
  );
}
