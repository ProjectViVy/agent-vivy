import { defineConfig } from "vite";

// The Go process embeds ui/dist via go:embed (single binary). The dist
// directory carries a committed .keep placeholder so the Go build works
// before the first frontend build; emptyOutDir must stay off to protect it.
export default defineConfig({
  build: {
    emptyOutDir: false,
  },
  server: {
    proxy: {
      "/api": "http://127.0.0.1:8787",
    },
  },
});
