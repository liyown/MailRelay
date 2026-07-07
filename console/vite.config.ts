import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import path from "node:path";

export default defineConfig({
  test: {
    environment: "jsdom",
    setupFiles: "./src/test/setup.ts",
  },
  optimizeDeps: {
    include: ["react", "react-dom/client"],
  },
  server: {
    warmup: {
      clientFiles: ["./src/main.tsx"],
    },
  },
  resolve: { alias: { "@": path.resolve(__dirname, "./src") } },
  build: {
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (id.includes("recharts") || id.includes("d3-")) return "charts";
          if (id.includes("@tanstack")) return "query";
          if (id.includes("radix-ui") || id.includes("@radix-ui")) return "ui";
          if (id.includes("react")) return "react";
        },
      },
    },
  },
  plugins: [react(), tailwindcss()],
});
