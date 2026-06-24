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
      // protoc-gen-es v2 uses package subpath exports that Rollup resolves via
      // the "import" condition; map them to their ESM dist paths explicitly so
      // the production build can bundle them.
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
