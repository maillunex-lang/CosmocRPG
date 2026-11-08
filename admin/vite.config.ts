/// <reference types="vitest" />
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// В разработке запросы /api проксируются на админ-API Go-сервера.
// В продакшене /api проксирует Caddy (infrastructure/).
export default defineConfig({
  plugins: [react()],
  server: { proxy: { "/api": "http://127.0.0.1:8081" } },
  test: { environment: "jsdom", globals: false, restoreMocks: true },
});
