import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// Builds one ES module, ../static/react/app.js. The Go server serves ./static
// at /static and the layout loads /static/react/app.js, so no dev server is
// needed: `npm run dev` just rebuilds on every change.
export default defineConfig({
  plugins: [react()],
  base: "/static/react/",
  build: {
    outDir: "../static/react",
    emptyOutDir: true,
    rollupOptions: {
      input: "src/main.tsx",
      output: {
        entryFileNames: "app.js",
        chunkFileNames: "chunks/[name]-[hash].js",
        assetFileNames: "[name][extname]",
      },
    },
  },
});
