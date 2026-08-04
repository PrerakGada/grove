import { type ReactNode } from "react";

export function Section({
  id,
  eyebrow,
  title,
  lede,
  children,
  className = "",
}: {
  id?: string;
  eyebrow?: string;
  title?: ReactNode;
  lede?: ReactNode;
  children?: ReactNode;
  className?: string;
}) {
  return (
    <section
      id={id}
      className={`scroll-mt-20 border-t border-line py-20 sm:py-28 ${className}`}
    >
      <div className="mx-auto max-w-6xl px-5 sm:px-8">
        {eyebrow ? (
          <p className="mb-4 font-mono text-xs tracking-[0.14em] text-ok">
            {eyebrow}
          </p>
        ) : null}
        {title ? (
          <h2 className="max-w-3xl text-3xl leading-[1.15] font-medium tracking-[-0.02em] text-balance sm:text-4xl">
            {title}
          </h2>
        ) : null}
        {lede ? (
          <p className="mt-4 max-w-2xl text-[17px] leading-relaxed text-fg-muted text-pretty">
            {lede}
          </p>
        ) : null}
        {children ? <div className="mt-12">{children}</div> : null}
      </div>
    </section>
  );
}
