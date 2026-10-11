import { nav, REPO } from "@/lib/content";
import { Wordmark, GitHubIcon } from "./Brand";

export function Nav({ stars }: { stars?: number }) {
  return (
    <header className="sticky top-0 z-50 border-b border-line/80 bg-bg/80 backdrop-blur-md">
      <div className="mx-auto flex h-14 max-w-6xl items-center gap-6 px-5 sm:px-8">
        <a
          href="#top"
          className="flex items-center gap-2.5 font-mono text-sm text-fg"
        >
          <Wordmark className="size-8" />
          grove
        </a>

        <nav className="ml-auto hidden items-center gap-6 md:flex">
          {nav.map((l) => (
            <a
              key={l.href}
              href={l.href}
              className="text-[13px] text-fg-muted transition-colors hover:text-fg"
            >
              {l.label}
            </a>
          ))}
        </nav>

        <a
          href={REPO}
          target="_blank"
          rel="noreferrer noopener"
          className="ml-auto flex items-center gap-2 rounded-md border border-line px-3 py-1.5 text-[13px] text-fg-muted transition-colors hover:border-line-strong hover:text-fg md:ml-0"
        >
          <GitHubIcon className="size-4" />
          <span>GitHub</span>
          {stars && stars > 0 ? (
            <span className="font-mono text-xs text-fg-dim tabular-nums">
              {stars}
            </span>
          ) : null}
        </a>
      </div>
    </header>
  );
}
