import { ImageResponse } from "next/og";
import { readFile } from "node:fs/promises";
import { join } from "node:path";

export const alt =
  "Grove — parallel dev environments from git worktrees. Reserved ports, own dependencies, own database, background runner.";
export const size = { width: 1200, height: 630 };
export const contentType = "image/png";

const BANDS = [
  { name: "web", band: 3100 },
  { name: "api", band: 8200 },
  { name: "db", band: 5500 },
];
const TAKEN = ["main", "payments", "auth-v2", "refactor"];

const BG = "#08090a";
const FG = "#e8ebed";
const MUTED = "#9aa4ab";
const DIM = "#757f86";
const LINE = "#1b1f23";
const OK = "#3fdb84";

export default async function Image() {
  const [regular, bold, logo] = await Promise.all([
    readFile(join(process.cwd(), "assets/JetBrainsMono-Regular.ttf")),
    readFile(join(process.cwd(), "assets/JetBrainsMono-Bold.ttf")),
    readFile(join(process.cwd(), "public/brand/grove-icon.png")),
  ]);

  return new ImageResponse(
    (
      <div
        style={{
          width: "100%",
          height: "100%",
          display: "flex",
          flexDirection: "column",
          justifyContent: "space-between",
          background: BG,
          padding: 64,
          fontFamily: "JetBrains Mono",
          borderTop: `6px solid ${OK}`,
        }}
      >
        <div style={{ display: "flex", flexDirection: "column" }}>
          <div style={{ display: "flex", alignItems: "center", gap: 14 }}>
            {/* ImageResponse embeds this local PNG; it does not use next/image. */}
            <img
              src={`data:image/png;base64,${logo.toString("base64")}`}
              width={48}
              height={48}
              alt=""
              style={{ borderRadius: 9 }}
            />
            <span style={{ color: FG, fontSize: 27 }}>grove</span>
            <span style={{ color: DIM, fontSize: 19, marginLeft: 8 }}>
              open source · go · mit
            </span>
          </div>

          <div
            style={{
              display: "flex",
              color: FG,
              fontSize: 61,
              fontWeight: 700,
              lineHeight: 1.15,
              marginTop: 44,
              letterSpacing: -2,
            }}
          >
            Parallel dev environments
          </div>
          <div
            style={{
              display: "flex",
              color: OK,
              fontSize: 61,
              fontWeight: 700,
              lineHeight: 1.15,
              letterSpacing: -2,
            }}
          >
            from git worktrees.
          </div>

          <div
            style={{
              display: "flex",
              color: MUTED,
              fontSize: 24,
              marginTop: 26,
              marginBottom: 8,
              lineHeight: 1.45,
            }}
          >
            Reserved ports · own dependencies · own database · background runner
          </div>
        </div>

        {/* the band table — the same numbers the page draws */}
        <div
          style={{
            display: "flex",
            flexDirection: "column",
            border: `1px solid ${LINE}`,
            borderRadius: 12,
            padding: "22px 26px",
            background: "#0d0f11",
          }}
        >
          <div style={{ display: "flex", fontSize: 19, color: DIM }}>
            <span style={{ width: 148 }}>slot</span>
            {TAKEN.map((_, i) => (
              <span key={i} style={{ width: 122 }}>
                {i}
              </span>
            ))}
            <span style={{ color: DIM }}>… 99</span>
          </div>

          {BANDS.map(({ name, band }) => (
            <div
              key={name}
              style={{ display: "flex", fontSize: 23, marginTop: 12 }}
            >
              <span style={{ width: 62, color: DIM }}>{name}</span>
              <span style={{ width: 86, color: MUTED }}>{band}</span>
              {TAKEN.map((_, i) => (
                <span key={i} style={{ width: 122, color: OK }}>
                  {band + i}
                </span>
              ))}
            </div>
          ))}

          <div
            style={{ display: "flex", fontSize: 19, color: DIM, marginTop: 16 }}
          >
            {TAKEN.map((n, i) => (
              <span
                key={n}
                style={{ width: 122, marginLeft: i === 0 ? 148 : 0 }}
              >
                {n}
              </span>
            ))}
          </div>
        </div>
      </div>
    ),
    {
      ...size,
      fonts: [
        { name: "JetBrains Mono", data: regular, style: "normal", weight: 400 },
        { name: "JetBrains Mono", data: bold, style: "normal", weight: 700 },
      ],
    },
  );
}
