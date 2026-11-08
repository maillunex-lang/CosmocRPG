/// <reference types="vitest" />
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// Админка живёт по адресу /admin/ (https://<хост>/admin/).
// В разработке запросы /admin/api/* проксируются на админ-API Go-сервера (без префикса /admin);
// в продакшене то же делает Caddy (infrastructure/caddy/Caddyfile).
export default defineConfig({
  base: "/admin/",
  plugins: [react()],
  server: {
    proxy: {
      "/admin/api": { target: "http://127.0.0.1:8081", rewrite: (p) => p.replace(/^\/admin/, "") },
    },
  },
  test: { environment: "jsdom", globals: false, restoreMocks: true },
});
