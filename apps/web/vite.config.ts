import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// Dev: `forge serve` runs the API on 127.0.0.1:8080, Vite proxies /api to it.
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: { "/api": { target: "http://127.0.0.1:8080", changeOrigin: true } },
  },
  build: { outDir: "dist", emptyOutDir: true },
});
