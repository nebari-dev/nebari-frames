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
      // gen/ts lives outside web/ so its @bufbuild/protobuf subpath imports
      // (codegenv2, wkt) cannot resolve via normal node_modules lookup.
      // These three aliases (vite.config.ts, vitest.config.ts, tsconfig.json)
      // are the required workaround. The proper long-term fix is to convert
      // this repo to an npm workspace so gen/ts shares web/'s node_modules.
      "@bufbuild/protobuf/codegenv2": path.resolve(
        __dirname,
        "./node_modules/@bufbuild/protobuf/dist/esm/codegenv2/index.js",
      ),
      "@bufbuild/protobuf/wkt": path.resolve(
        __dirname,
        "./node_modules/@bufbuild/protobuf/dist/esm/wkt/index.js",
      ),
    },
  },
  server: {
    proxy: {
      "/frames.v1.FrameService": { target: BACKEND, changeOrigin: true },
      "/auth/config": { target: BACKEND, changeOrigin: true },
    },
  },
});
