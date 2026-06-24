import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import path from "node:path";

// RPC + auth config are proxied to the Go backend in dev so the browser
// talks to a single origin (mirrors the embedded production deployment).
const BACKEND = process.env.VITE_BACKEND_URL ?? "http://localhost:8080";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
      "@gen": path.resolve(__dirname, "../gen/ts"),
    },
  },
  server: {
    proxy: {
      "/frames.v1.FrameService": { target: BACKEND, changeOrigin: true },
      "/auth/config": { target: BACKEND, changeOrigin: true },
    },
  },
});
