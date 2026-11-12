import { networkInterfaces } from "node:os";
import { resolve } from "node:path";
import type { NextConfig } from "next";
import { loadConfigDotEnv } from "./config/load-env.mjs";

loadConfigDotEnv(resolve(__dirname, "config/.env"));

// Next 16 blocks cross-origin access to /_next dev resources unless the browser
// host is listed. "*" is NOT a wildcard — use explicit hosts. This machine's own
// addresses are always listed, so another computer on the LAN can open the dev server.
function allowedDevOrigins(): string[] {
  const hosts = new Set<string>(["127.0.0.1", "localhost"]);
  for (const list of Object.values(networkInterfaces())) {
    for (const a of list ?? []) if (a.family === "IPv4" && !a.internal) hosts.add(a.address);
  }
  for (const part of (process.env.DEV_ALLOWED_ORIGINS ?? "").split(",")) {
    const h = part.trim();
    if (h) hosts.add(h);
  }
  try {
    const api = process.env.API_URL;
    if (api) {
      const host = new URL(api).hostname;
      if (host && host !== "0.0.0.0") hosts.add(host);
    }
  } catch {
    /* ignore bad API_URL */
  }
  return [...hosts];
}

const nextConfig: NextConfig = {
  output: "standalone",
  poweredByHeader: false,
  async headers() {
    const dev = process.env.NODE_ENV !== "production";
    const csp = [
      "default-src 'self'",
      // Pluggy Connect: its widget runs in a frame of connect.pluggy.ai; bank logos come from its CDN.
      `script-src 'self' 'unsafe-inline'${dev ? " 'unsafe-eval'" : ""}`,
      "frame-src https://connect.pluggy.ai",
      "style-src 'self' 'unsafe-inline'",
      "img-src 'self' data: blob: https://*.pluggy.ai",
      "font-src 'self' data:",
      `connect-src 'self'${dev ? " ws: wss:" : ""}`,
      "worker-src 'self' blob:",
      "object-src 'none'",
      "base-uri 'self'",
      "frame-ancestors 'none'",
    ].join("; ");
    return [
      {
        source: "/:path*",
        headers: [
          { key: "Content-Security-Policy", value: csp },
          { key: "X-Content-Type-Options", value: "nosniff" },
          { key: "X-Frame-Options", value: "DENY" },
          { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
          {
            key: "Permissions-Policy",
            value: "camera=(), microphone=(), geolocation=()",
          },
        ],
      },
    ];
  },
  allowedDevOrigins: allowedDevOrigins(),
};

export default nextConfig;
