import { defineConfig } from "mygo-cli";

export default defineConfig({
  name: "media-fetcher",
  identifier: "com.media.fetcher",
  version: "1.0.0",
  devUrl: "http://localhost:5173",
  devCommand: "pnpm run dev",
  buildCommand: "pnpm run build",
  frontendDist: "dist",
  bindings: "src/mygo.ts",
  out: "build",
});
