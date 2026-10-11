import type { MetadataRoute } from "next";

export default function manifest(): MetadataRoute.Manifest {
  return {
    name: "Grove — Git worktree environments",
    short_name: "Grove",
    description: "Parallel dev environments from Git worktrees.",
    start_url: "/",
    scope: "/",
    display: "browser",
    background_color: "#ffffff",
    theme_color: "#08090a",
    icons: [
      { src: "/brand/icon-192.png", sizes: "192x192", type: "image/png", purpose: "any" },
      { src: "/brand/icon-512.png", sizes: "512x512", type: "image/png", purpose: "any" },
      { src: "/brand/maskable-192.png", sizes: "192x192", type: "image/png", purpose: "maskable" },
      { src: "/brand/maskable-512.png", sizes: "512x512", type: "image/png", purpose: "maskable" },
    ],
  };
}
