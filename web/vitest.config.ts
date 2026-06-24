import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import path from "node:path";

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
      "@gen": path.resolve(__dirname, "../gen/ts"),
      "@bufbuild/protobuf/codegenv2": path.resolve(__dirname, "./node_modules/@bufbuild/protobuf/dist/esm/codegenv2/index.js"),
      "@bufbuild/protobuf/wkt": path.resolve(__dirname, "./node_modules/@bufbuild/protobuf/dist/esm/wkt/index.js"),
    },
  },
  test: {
    globals: true,
    environment: "jsdom",
    setupFiles: ["./src/test/setup.ts"],
  },
});
