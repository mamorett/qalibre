import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  base: "/spa/",
  server: {
    port: 5173,
    // Proxy API calls to the Flask app during `vite dev`
    proxy: {
      "/api":      { target: "http://localhost:8083", changeOrigin: true },
      "/ajax":     { target: "http://localhost:8083", changeOrigin: true },
      "/get_":     { target: "http://localhost:8083", changeOrigin: true },
      "/cover":    { target: "http://localhost:8083", changeOrigin: true },
      "/opds":     { target: "http://localhost:8083", changeOrigin: true },
      "/read":     { target: "http://localhost:8083", changeOrigin: true },
      "/download": { target: "http://localhost:8083", changeOrigin: true },
    },
  },
  build: { outDir: "dist", sourcemap: true },
});

