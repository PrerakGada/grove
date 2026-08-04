import { BandRuler } from "@/components/BandRuler";
import { Wordmark, GitHubIcon } from "@/components/Brand";
import { CopyCommand } from "@/components/CopyCommand";
import { Lifecycle } from "@/components/Lifecycle";
import { Nav } from "@/components/Nav";
import { Section } from "@/components/Section";
import { Terminal, TerminalBody } from "@/components/Terminal";
import {
  INSTALL,
  REPO,
  commands,
  configExample,
  failures,
  honest,
  lifecycle,
  pillars,
  stats,
  term,
} from "@/lib/content";

async function stars(): Promise<number | undefined> {
  try {
    const r = await fetch("https://api.github.com/repos/PrerakGada/grove", {
      next: { revalidate: 3600 },
    });
    if (!r.ok) return undefined;
    const j = (await r.json()) as { stargazers_count?: number };
    return j.stargazers_count;
  } catch {
    return undefined;
  }
}

export default async function Home() {
  const count = await stars();

  return (
    <>
      <Nav stars={count} />

      <main id="top">
        {/* ------------------------------------------------------------ hero */}
        <section className="relative overflow-hidden">
          <div
            className="bg-grid mask-fade-b pointer-events-none absolute inset-0 opacity-[0.55]"
            aria-hidden="true"
          />
          <div
            className="pointer-events-none absolute -top-40 left-1/2 h-[30rem] w-[60rem] -translate-x-1/2 rounded-full opacity-[0.07] blur-3xl"
            style={{ background: "var(--color-ok)" }}
            aria-hidden="true"
          />

          <div className="relative mx-auto max-w-6xl px-5 pt-20 pb-24 sm:px-8 sm:pt-28 sm:pb-32">
            <p className="mb-6 flex flex-wrap items-center gap-x-3 gap-y-2 font-mono text-xs tracking-[0.14em] text-fg-dim">
              <span>OPEN SOURCE</span>
              <span className="text-line-strong">/</span>
              <span>GO</span>
              <span className="text-line-strong">/</span>
              <span>MIT</span>
              <span className="text-line-strong">/</span>
              <span className="text-warn">PRE-1.0</span>
            </p>

            <h1 className="max-w-4xl text-4xl leading-[1.08] font-medium tracking-[-0.03em] text-balance sm:text-6xl">
              Every worktree, a <span className="text-ok">real environment</span>
              .
            </h1>

            <p className="mt-6 max-w-2xl text-lg leading-relaxed text-fg-muted text-pretty sm:text-xl">
              Grove gives each git worktree reserved ports, its own
              dependencies, its own database and a background runner. Eight
              branches running at once, and not one of them holding a terminal.
            </p>

            <div className="mt-9 flex flex-col gap-3 sm:flex-row sm:items-center">
              <CopyCommand command={INSTALL} />
              <a
                href={REPO}
                target="_blank"
                rel="noreferrer noopener"
                className="flex items-center justify-center gap-2 rounded-lg border border-line-strong px-4 py-3 text-sm font-medium text-fg transition-colors hover:bg-surface"
              >
                <GitHubIcon className="size-4" />
                Read the source
              </a>
            </div>

            <p className="mt-4 font-mono text-xs text-fg-dim">
              Go 1.26+ · macOS and Linux · Windows via WSL2
            </p>

            <dl className="mt-10 flex flex-wrap gap-x-10 gap-y-5">
              {stats.map(([n, label]) => (
                <div key={label}>
                  <dt className="sr-only">{label}</dt>
                  <dd>
                    <span className="block font-mono text-2xl text-fg tabular-nums">
                      {n}
                    </span>
                    <span className="mt-0.5 block font-mono text-[11px] tracking-[0.1em] text-fg-dim">
                      {label.toUpperCase()}
                    </span>
                  </dd>
                </div>
              ))}
            </dl>

            <div className="mt-16 grid gap-5 *:min-w-0 lg:grid-cols-[1.05fr_1fr] lg:items-start">
              <BandRuler />
              <Terminal title="grove ports">
                <TerminalBody text={term.ports} />
              </Terminal>
            </div>
          </div>
        </section>

        {/* --------------------------------------------------------- pillars */}
        <section className="border-t border-line" aria-labelledby="pillars-h">
          {/* keeps the h1 → h3 jump from being a heading-order skip */}
          <h2 id="pillars-h" className="sr-only">
            What Grove gives every worktree
          </h2>
          <div className="mx-auto grid max-w-6xl gap-px bg-line sm:grid-cols-2 lg:grid-cols-4">
            {pillars.map((p) => (
              <div key={p.k} className="bg-bg p-7 sm:p-8">
                <p className="font-mono text-xs tracking-[0.14em] text-ok">
                  {p.k.toUpperCase()}
                </p>
                <h3 className="mt-4 text-[17px] font-medium tracking-[-0.01em]">
                  {p.h}
                </h3>
                <p className="mt-2.5 text-[14px] leading-relaxed text-fg-muted">
                  {p.b}
                </p>
              </div>
            ))}
          </div>
        </section>

        {/* ----------------------------------------------------------- ports */}
        <Section
          id="ports"
          eyebrow="PORTS"
          title="One offset, applied to every band."
          lede="Each service owns a 100-wide band. A worktree owns one slot, and the main checkout is slot 0 — so a band is also its own port, and the trailing digits of any port are always the slot number. Ninety-nine worktrees fit before a band is full."
        >
          <div className="space-y-5">
            <Terminal title="grove list">
              <TerminalBody text={term.list} />
            </Terminal>
            <div className="grid gap-5 *:min-w-0 sm:grid-cols-2">
              <div className="rounded-xl border border-line bg-surface/60 p-6">
                <h3 className="font-mono text-sm text-fg">
                  Bands are reserved globally
                </h3>
                <p className="mt-3 text-[14px] leading-relaxed text-fg-muted">
                  Reservations live in{" "}
                  <code className="text-key">~/.grove</code>, not in the
                  repository — so a second project is given a different band
                  rather than the one already taken, and Grove will not reserve
                  a colliding band unless you confirm it. Every allocation is a
                  flock-guarded read-modify-write with an atomic rename.
                </p>
              </div>
              <div className="rounded-xl border border-line bg-surface/60 p-6">
                <h3 className="font-mono text-sm text-fg">
                  Well-known ports are called out, not silently moved
                </h3>
                <p className="mt-3 text-[14px] leading-relaxed text-fg-muted">
                  A band containing 3000, 5173 or 5432 only bites at the one
                  slot whose offset lands there. Grove warns and names that
                  slot rather than relocating a band your project already uses.
                  A collision with another <em>project</em> is graver:{" "}
                  <code className="text-fg">grove init -y</code> refuses
                  outright, and interactively you have to confirm it.
                </p>
              </div>
            </div>
          </div>
        </Section>

        {/* ------------------------------------------------------- lifecycle */}
        <Section
          id="lifecycle"
          eyebrow="LIFECYCLE"
          title="Five commands, from empty repo to eight running branches."
          lede="Every block below is literal stdout, captured from the binary against a scratch pnpm monorepo."
        >
          <Lifecycle
            tabs={lifecycle.map(({ cmd, label }) => ({ cmd, label }))}
            panels={lifecycle.map((s) => (
              <div key={s.cmd}>
                <p className="mb-4 max-w-2xl text-[15px] leading-relaxed text-fg-muted">
                  {s.blurb}
                </p>
                <Terminal title={s.cmd}>
                  <TerminalBody text={s.term} />
                </Terminal>
              </div>
            ))}
          />
        </Section>

        {/* ---------------------------------------------------------- config */}
        <Section
          id="config"
          eyebrow="CONFIGURATION"
          title="Two files, and one rule between them."
          lede="Nothing machine-specific may reach git, and nothing project-shaped may live only on your machine. A teammate clones the repo and grove works without editing anything."
        >
          <div className="space-y-5">
            <div className="grid gap-5 *:min-w-0 lg:grid-cols-3">
              <div className="rounded-xl border border-line bg-surface/60 p-6">
                <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-2">
                  <code className="font-mono text-sm text-key">
                    .grove.json
                  </code>
                  <span className="rounded border border-ok/30 px-2 py-0.5 font-mono text-[10px] tracking-wide text-ok">
                    COMMITTED
                  </span>
                </div>
                <p className="mt-3 text-[14px] leading-relaxed text-fg-muted">
                  The project&apos;s shape — services, port bands, package
                  manager, database, env files. Written by{" "}
                  <code className="text-fg">grove init</code> after it inspects
                  the repo, and edited by hand thereafter. It is the contract;
                  detection is only a convenience.
                </p>
              </div>
              <div className="rounded-xl border border-line bg-surface/60 p-6">
                <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-2">
                  <code className="min-w-0 font-mono text-sm break-all text-key">
                    ~/.grove/registry.json
                  </code>
                  <span className="rounded border border-line-strong px-2 py-0.5 font-mono text-[10px] tracking-wide text-fg-dim">
                    NEVER COMMITTED
                  </span>
                </div>
                <p className="mt-3 text-[14px] leading-relaxed text-fg-muted">
                  This machine&apos;s allocation — which project owns which
                  band, which worktree holds which slot. Two people on the same
                  project get different slots without coordinating.
                </p>
              </div>
              <div className="rounded-xl border border-line bg-surface/60 p-6">
                <h3 className="font-mono text-sm text-fg">
                  Values are templated, never substituted
                </h3>
                <p className="mt-3 text-[14px] leading-relaxed text-fg-muted">
                  <code className="text-key">{"{{port.api}}"}</code>,{" "}
                  <code className="text-key">{"{{slot}}"}</code>,{" "}
                  <code className="text-key">{"{{db.name}}"}</code>. An unknown
                  variable is a hard error, so a typo fails at provisioning time
                  instead of writing itself into a{" "}
                  <code className="text-fg">.env</code>.
                </p>
              </div>
            </div>

            <Terminal title=".grove.json">
              <pre className="min-w-max px-4 py-4 font-mono text-[12.5px] leading-[1.7] text-fg-muted">
                {configExample}
              </pre>
            </Terminal>
          </div>
        </Section>

        {/* -------------------------------------------------------- failures */}
        <Section
          id="failures"
          eyebrow="WHY IT LOOKS LIKE THIS"
          title="Grove is a rewrite. The bash version is the specification."
          lede="Eighteen hundred lines of shell ran this daily, on a machine with eight worktrees, until each of these broke it. Every non-obvious decision in the Go version is here because the obvious one was tried first."
        >
          <ol className="grid gap-px bg-line sm:grid-cols-2">
            {failures.map((f) => (
              <li key={f.n} className="bg-bg p-7 sm:p-8">
                <div className="flex items-baseline gap-3">
                  <span className="font-mono text-xs text-fg-dim tabular-nums">
                    {f.n}
                  </span>
                  <h3 className="text-[17px] font-medium tracking-[-0.01em]">
                    {f.title}
                  </h3>
                </div>
                <a
                  href={`${REPO}/blob/main/${f.src}`}
                  target="_blank"
                  rel="noreferrer noopener"
                  className="mt-2 ml-[1.7rem] inline-block font-mono text-[11px] text-fg-dim underline decoration-line-strong underline-offset-4 transition-colors hover:text-key"
                >
                  {f.src}
                </a>

                <dl className="mt-5 space-y-3.5 text-[14px] leading-relaxed">
                  <div className="flex gap-3">
                    <dt className="w-14 shrink-0 font-mono text-[11px] text-fg-dim">
                      naive
                    </dt>
                    <dd className="text-fg-muted">{f.naive}</dd>
                  </div>
                  <div className="flex gap-3">
                    <dt className="w-14 shrink-0 font-mono text-[11px] text-err">
                      broke
                    </dt>
                    <dd className="text-fg-muted">{f.broke}</dd>
                  </div>
                  <div className="flex gap-3">
                    <dt className="w-14 shrink-0 font-mono text-[11px] text-ok">
                      grove
                    </dt>
                    <dd className="text-fg">{f.fix}</dd>
                  </div>
                </dl>
              </li>
            ))}
          </ol>

          <div className="mt-5 space-y-5">
            <Terminal title="the guard that fires">
              <TerminalBody text={term.guard} />
            </Terminal>
            <Terminal title="grove doctor">
              <TerminalBody text={term.doctor} />
            </Terminal>
          </div>
        </Section>

        {/* -------------------------------------------------------- commands */}
        <Section
          eyebrow="REFERENCE"
          title="Fifteen commands."
          lede="Nothing hidden behind a plugin system."
        >
          {/* a definition list rather than a table: it stacks on a phone
              instead of forcing a 392px scroller into a 278px viewport, and
              the command itself still never wraps mid-token */}
          <dl className="overflow-hidden rounded-xl border border-line">
            {commands.map(([c, d], i) => (
              <div
                key={c}
                className={`gap-x-5 px-5 py-3 sm:grid sm:grid-cols-[minmax(0,15rem)_1fr] ${
                  i % 2 ? "bg-surface/40" : "bg-transparent"
                } ${i ? "border-t border-line" : ""}`}
              >
                <dt className="font-mono text-[13px] whitespace-nowrap text-fg">
                  <span className="text-fg-dim">grove </span>
                  {c}
                </dt>
                <dd className="mt-1 font-mono text-[13px] text-fg-muted sm:mt-0">
                  {d}
                </dd>
              </div>
            ))}
          </dl>
        </Section>

        {/* ---------------------------------------------------------- honest */}
        <Section
          eyebrow="WHERE IT IS NOT DONE"
          title="Pre-1.0, and specific about it."
          lede="In daily use on two real projects — a pnpm/Turbo monorepo with six services and Postgres, and a polyglot repo tying a Flutter app to a NestJS API. Here is what it still is not."
        >
          <div className="grid gap-px bg-line sm:grid-cols-2">
            {honest.map((h) => (
              <div key={h.h} className="bg-bg p-7 sm:p-8">
                <h3 className="flex items-baseline gap-2.5 text-[16px] font-medium">
                  <span className="font-mono text-warn" aria-hidden="true">
                    !
                  </span>
                  {h.h}
                </h3>
                <p className="mt-2.5 text-[14px] leading-relaxed text-fg-muted">
                  {h.b}
                </p>
              </div>
            ))}
          </div>
        </Section>

        {/* ------------------------------------------------------------- cta */}
        <section id="install" className="scroll-mt-20 border-t border-line">
          <div className="relative overflow-hidden">
            <div
              className="bg-grid pointer-events-none absolute inset-0 opacity-40"
              aria-hidden="true"
            />
            <div className="relative mx-auto max-w-6xl px-5 py-24 text-center sm:px-8 sm:py-32">
              <Wordmark className="mx-auto size-8 text-ok" />
              <h2 className="mx-auto mt-6 max-w-2xl text-3xl leading-[1.15] font-medium tracking-[-0.02em] text-balance sm:text-4xl">
                Stop stopping servers to change branches.
              </h2>
              <p className="mx-auto mt-4 max-w-xl text-[17px] leading-relaxed text-fg-muted text-pretty">
                One command to install, one to detect your repo, one per branch
                after that.
              </p>

              <div className="mx-auto mt-9 flex max-w-xl flex-col gap-3">
                <CopyCommand command={INSTALL} />
                <CopyCommand command="grove init" />
              </div>

              <a
                href={REPO}
                target="_blank"
                rel="noreferrer noopener"
                className="mt-6 inline-flex items-center gap-2 text-sm text-fg-muted transition-colors hover:text-fg"
              >
                <GitHubIcon className="size-4" />
                github.com/PrerakGada/grove
              </a>
            </div>
          </div>
        </section>
      </main>

      <footer className="border-t border-line">
        {/* the key to every terminal block above — the CLI's own four marks */}
        <div className="mx-auto max-w-6xl px-5 pt-8 sm:px-8">
          <div className="flex flex-wrap items-center gap-x-6 gap-y-2 font-mono text-[11px] text-fg-dim">
            <span className="tracking-[0.14em]">MARKS</span>
            <span>
              <span className="text-ok">✓</span> done
            </span>
            <span>
              <span className="text-info">→</span> next
            </span>
            <span>
              <span className="text-warn">!</span> warning
            </span>
            <span>
              <span className="text-err">✗</span> error, stderr, exit 1
            </span>
          </div>
        </div>

        <div className="mx-auto flex max-w-6xl flex-col gap-4 px-5 py-8 sm:flex-row sm:items-center sm:justify-between sm:px-8">
          <p className="font-mono text-xs text-fg-dim">
            MIT · built by{" "}
            <a
              href="https://prerakgada.in"
              target="_blank"
              rel="noreferrer noopener"
              className="text-fg-muted transition-colors hover:text-fg"
            >
              Prerak Gada
            </a>
          </p>
          <div className="flex items-center gap-6 font-mono text-xs">
            <a
              href={REPO}
              target="_blank"
              rel="noreferrer noopener"
              className="text-fg-dim transition-colors hover:text-fg"
            >
              Source
            </a>
            <a
              href={`${REPO}/issues`}
              target="_blank"
              rel="noreferrer noopener"
              className="text-fg-dim transition-colors hover:text-fg"
            >
              Issues
            </a>
            <a
              href={`${REPO}/blob/main/LICENSE`}
              target="_blank"
              rel="noreferrer noopener"
              className="text-fg-dim transition-colors hover:text-fg"
            >
              License
            </a>
          </div>
        </div>
      </footer>
    </>
  );
}
