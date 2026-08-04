"use client";

import { useEffect, useLayoutEffect, useRef, useState } from "react";

/*
  The one idea worth drawing — and worth letting the reader break.

  Each service owns a 100-wide band. A worktree owns one slot, and that same
  offset is applied to every band at once, so a worktree is a *column* here and
  reading down it gives the same trailing digit three times.

  The toggle runs the predecessor's formula in the same coordinate space:
  band + slot*100 leaves its own band at slot 1, and at slot 24 the web band
  lands exactly on the database's. That is the 24-worktree ceiling, drawn
  rather than asserted.
*/

const BANDS = [
  { name: "web", band: 3100 },
  { name: "api", band: 8200 },
  { name: "db", band: 5500 },
] as const;

const TAKEN = ["main", "payments", "auth-v2", "refactor"] as const;

/* Six columns is the most that still fits "payments" un-truncated beside the
   terminal in the hero. Slots 6-99 stay implied by the end cap. */
const COLS = 6;

const useIsoLayoutEffect =
  typeof window !== "undefined" ? useLayoutEffect : useEffect;

const port = (band: number, slot: number, naive: boolean) =>
  naive ? band + slot * 100 : band + slot;

const inBand = (band: number, value: number) =>
  value >= band && value <= band + 99;

export function BandRuler() {
  const ref = useRef<HTMLDivElement>(null);
  /* Rendered full on the server: a crawler or a reader with JS off sees the
     finished diagram, not an empty one. The animation resets it before paint. */
  const [shown, setShown] = useState<number>(TAKEN.length);
  const [active, setActive] = useState<number | null>(null);
  const [naive, setNaive] = useState(false);

  useIsoLayoutEffect(() => {
    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
    setShown(0);
  }, []);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;

    const io = new IntersectionObserver(
      ([e]) => {
        if (!e.isIntersecting) return;
        io.disconnect();
        TAKEN.forEach((_, i) =>
          setTimeout(() => setShown(i + 1), 260 + i * 380),
        );
      },
      { threshold: 0.25 },
    );
    io.observe(el);
    return () => io.disconnect();
  }, []);

  const readout = active ?? Math.max(shown - 1, 0);

  return (
    <div
      ref={ref}
      className="rounded-xl border border-line bg-surface/60 p-5 sm:p-6"
    >
      <div className="mb-5 flex flex-col items-start gap-3 sm:flex-row sm:items-center sm:justify-between">
        <span className="font-mono text-xs tracking-[0.14em] text-fg-dim">
          {Math.max(shown - 1, 0)} OF 99 SLOTS
        </span>

        <div
          className="flex rounded-md border border-line p-0.5 font-mono text-[11px]"
          role="group"
          aria-label="Port allocation formula"
        >
          <button
            onClick={() => setNaive(false)}
            aria-pressed={!naive}
            className={`rounded-sm px-2.5 py-1 transition-colors ${
              !naive ? "bg-ok/15 text-ok" : "text-fg-dim hover:text-fg-muted"
            }`}
          >
            band + slot
          </button>
          <button
            onClick={() => setNaive(true)}
            aria-pressed={naive}
            className={`rounded-sm px-2.5 py-1 transition-colors ${
              naive ? "bg-err/15 text-err" : "text-fg-dim hover:text-fg-muted"
            }`}
          >
            band + slot×100
          </button>
        </div>
      </div>

      <div className="overflow-x-auto">
        <div
          className="grid min-w-[27rem] items-center gap-y-1.5 font-mono text-[11px]"
          style={{
            gridTemplateColumns: `2rem 2.6rem repeat(${COLS}, minmax(0,1fr)) 2.6rem`,
          }}
          role="img"
          aria-label={
            naive
              ? "The same four worktrees under the predecessor's formula, band plus slot times 100: every port past slot 0 leaves its own 100-wide band."
              : "Three port bands — web 3100, api 8200, db 5500 — each 100 ports wide. Four worktrees hold slots 0 to 3. Slot 1 is 3101, 8201 and 5501: the same trailing digit in every band."
          }
        >
          <span />
          <span />
          {Array.from({ length: COLS }, (_, s) => (
            <span
              key={`n${s}`}
              onMouseEnter={() => s < TAKEN.length && setActive(s)}
              onMouseLeave={() => setActive(null)}
              className="truncate px-1 text-center text-[10px] transition-opacity duration-500"
              style={{
                opacity: s < shown ? 1 : 0,
                color:
                  active === s ? "var(--color-ok)" : "var(--color-fg-muted)",
              }}
            >
              {TAKEN[s] ?? ""}
            </span>
          ))}
          <span />

          <span />
          <span className="text-right text-fg-dim">slot</span>
          {Array.from({ length: COLS }, (_, s) => (
            <span
              key={`s${s}`}
              className="text-center tabular-nums transition-colors"
              style={{
                color:
                  active === s
                    ? "var(--color-ok)"
                    : s < shown
                      ? "var(--color-fg-muted)"
                      : "var(--color-fg-dim)",
              }}
            >
              {s}
            </span>
          ))}
          <span className="text-center text-fg-dim">99</span>

          {BANDS.map(({ name, band }) => (
            <Row
              key={name}
              name={name}
              band={band}
              shown={shown}
              active={active}
              naive={naive}
              setActive={setActive}
            />
          ))}
        </div>
      </div>

      <p
        className="mt-4 border-t border-line pt-4 font-mono text-[11px] leading-relaxed"
        aria-live="polite"
      >
        {naive ? (
          <span className="text-fg-muted">
            <span className="text-err">✗</span> Every port past slot 0 has left
            its own band. At slot 24, web reaches{" "}
            <span className="text-err">5500</span>
            {" — the database's band. That is the ceiling this replaced."}
          </span>
        ) : (
          <span className="text-fg-muted">
            <span className="text-ok">✓</span> Every port stays inside its band,
            and the trailing digits are the slot.{" "}
            <span className="text-fg-dim">
              Reading down a column gives one worktree.
            </span>
          </span>
        )}
      </p>

      <div className="mt-3 flex flex-wrap items-center gap-x-5 gap-y-2 font-mono text-xs">
        <span className="text-fg-dim">
          slot <span className="text-fg tabular-nums">{readout}</span>
        </span>
        {BANDS.map(({ name, band }) => {
          const v = port(band, readout, naive);
          const ok = inBand(band, v);
          return (
            <span key={name} className="text-fg-muted">
              {name}
              <span className="text-fg-dim">:</span>
              <span
                className="tabular-nums"
                style={{ color: ok ? "var(--color-ok)" : "var(--color-err)" }}
              >
                {v}
              </span>
            </span>
          );
        })}
      </div>
    </div>
  );
}

function Row({
  name,
  band,
  shown,
  active,
  naive,
  setActive,
}: {
  name: string;
  band: number;
  shown: number;
  active: number | null;
  naive: boolean;
  setActive: (n: number | null) => void;
}) {
  return (
    <>
      <span className="text-fg-dim">{name}</span>
      <span className="text-right text-fg-muted tabular-nums">{band}</span>
      {Array.from({ length: COLS }, (_, s) => {
        const on = s < shown;
        const hot = active === s;
        const value = port(band, s, naive);
        const ok = inBand(band, value);
        const text = String(value);

        return (
          <span
            key={s}
            onMouseEnter={() => s < TAKEN.length && setActive(s)}
            onMouseLeave={() => setActive(null)}
            className="py-1 text-center tabular-nums transition-all duration-500 ease-[var(--ease-term)]"
            style={{
              background: on
                ? ok
                  ? `color-mix(in oklab, var(--color-ok) ${hot ? 16 : 8}%, transparent)`
                  : `color-mix(in oklab, var(--color-err) ${hot ? 20 : 11}%, transparent)`
                : "transparent",
              opacity: on ? 1 : 0.55,
            }}
          >
            {on ? (
              ok ? (
                /* band prefix stays quiet; the slot digits carry the accent */
                <>
                  <span className="text-fg-muted">{text.slice(0, -1)}</span>
                  <span style={{ color: "var(--color-ok)" }}>
                    {text.slice(-1)}
                  </span>
                </>
              ) : (
                <span style={{ color: "var(--color-err)" }}>{text}</span>
              )
            ) : (
              <span className="text-fg-dim">·</span>
            )}
          </span>
        );
      })}
      <span className="text-center text-fg-dim tabular-nums">{band + 99}</span>
    </>
  );
}
