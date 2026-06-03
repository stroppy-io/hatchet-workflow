import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import path from "path";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  server: {
    port: 3000,
    proxy: {
      // connect-rpc: the client posts to /cloud.v1.<pkg>.<Service>/<Method>
      // (api + agent-log). Forward to the backend so dev stays same-origin and
      // needs no CORS — the prod build is served by that same backend on one mux.
      "/cloud.v1.": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
      // package blob serving — storage_uri download links resolve here.
      "/packages": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
    },
  },
});
