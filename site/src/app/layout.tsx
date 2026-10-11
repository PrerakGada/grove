import type { Metadata, Viewport } from "next";
import { Inter_Tight, JetBrains_Mono } from "next/font/google";
import { INSTALL } from "@/lib/content";
import "./globals.css";

const interTight = Inter_Tight({
  variable: "--font-inter-tight",
  subsets: ["latin"],
  display: "swap",
});

/* Mono is only ever rendered at 400 on this page, so pinning the weight ships
   one static instance instead of the full variable range. */
const jetbrainsMono = JetBrains_Mono({
  variable: "--font-jetbrains-mono",
  subsets: ["latin"],
  display: "swap",
  weight: ["400"],
});

const siteUrl = process.env.NEXT_PUBLIC_SITE_URL
  ? `https://${process.env.NEXT_PUBLIC_SITE_URL.replace(/^https?:\/\//, "")}`
  : process.env.VERCEL_PROJECT_PRODUCTION_URL
    ? `https://${process.env.VERCEL_PROJECT_PRODUCTION_URL}`
    : "http://localhost:4310";

const title = "Grove — parallel dev environments from git worktrees";
const description =
  "Every git worktree gets reserved ports, its own dependencies, its own database and a background runner. Run many branches at once without holding a terminal for each.";

export const metadata: Metadata = {
  metadataBase: new URL(siteUrl),
  title: { default: title, template: "%s · Grove" },
  description,
  applicationName: "Grove",
  keywords: [
    "git worktree",
    "parallel development",
    "dev environment",
    "port allocation",
    "monorepo",
    "cli",
    "go",
    "developer tools",
  ],
  authors: [{ name: "Prerak Gada", url: "https://prerakgada.in" }],
  creator: "Prerak Gada",
  openGraph: {
    type: "website",
    url: siteUrl,
    siteName: "Grove",
    title,
    description,
    locale: "en_US",
  },
  twitter: {
    card: "summary_large_image",
    title,
    description,
    images: [{ url: "/opengraph-image", width: 1200, height: 630, alt: "Grove — parallel dev environments from Git worktrees" }],
  },
  robots: {
    index: true,
    follow: true,
    googleBot: { index: true, follow: true, "max-image-preview": "large" },
  },
  alternates: { canonical: "/" },
};

export const viewport: Viewport = {
  themeColor: "#08090a",
  colorScheme: "dark",
};

export default function RootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <html
      lang="en"
      data-scroll-behavior="smooth"
      className={`${interTight.variable} ${jetbrainsMono.variable} h-full`}
    >
      <head>
        <link rel="mask-icon" href="/brand/safari-pinned-tab.svg" color="#3fdb84" />
        {/* The shared cookieless counter for product pages
            (~/work/company/infra/product-analytics.md). data-form turns on its
            optional "who is installing" form; the Install buttons carry
            data-pt-command, and data-command is what the form shows after it.
            Bump ?v= whenever the tracker changes: Cloudflare caches it. */}
        <script
          defer
          src="https://api.prerakgada.in/v1/p/tracker.js?v=5"
          data-product="grove"
          data-form=""
          data-name="Grove"
          data-accent="#3fdb84"
          data-command-label="Go"
          data-command={INSTALL}
        />
      </head>
      <body className="min-h-full antialiased">{children}</body>
    </html>
  );
}
