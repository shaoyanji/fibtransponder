// @ts-check
import { defineConfig } from 'astro/config';

/**
 * Static output to portfolio/dist, deployed to GitHub Pages by
 * .github/workflows/pages.yml.
 *
 * `base` matters: this is a project page at
 * github.io/shaoyanji/fibtransponder/, not a user page, so every absolute
 * asset path needs the repository prefix. The loader in src/lib/fib.ts derives
 * the wasm path from import.meta.env.BASE_URL for the same reason.
 */
const repo = 'fibtransponder';

export default defineConfig({
  site: `https://shaoyanji.github.io`,
  base: `/${repo}/`,
  output: 'static',
  trailingSlash: 'ignore',
  build: {
    // The site is a handful of small pages. Inlining anything would only
    // duplicate the fonts link and the wasm path.
    inlineStylesheets: 'auto',
  },
  vite: {
    build: {
      // The shim is loaded from public/ and never imported, so nothing here
      // needs chunking. Keep the warning threshold honest rather than silent.
      chunkSizeWarningLimit: 600,
    },
  },
});
