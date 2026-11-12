import type { MetadataRoute } from "next";
import { APP_NAME } from "@/lib/brand";

// Lets phones and desktops install Fin as an app (over HTTPS, see deploy/Caddyfile).
export default function manifest(): MetadataRoute.Manifest {
  return {
    name: APP_NAME,
    short_name: APP_NAME,
    description: "Finanças pessoais",
    start_url: "/",
    display: "standalone",
    background_color: "#ffffff",
    theme_color: "#3658c9",
    lang: "pt-BR",
    icons: [
      { src: "/icon-192.png", sizes: "192x192", type: "image/png", purpose: "any" },
      { src: "/icon-512.png", sizes: "512x512", type: "image/png", purpose: "any" },
      { src: "/icon-512.png", sizes: "512x512", type: "image/png", purpose: "maskable" },
    ],
  };
}
