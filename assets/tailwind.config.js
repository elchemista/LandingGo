import { defineConfig } from "tailwindcss";
import daisyui from "daisyui";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

const here = fileURLToPath(new URL(".", import.meta.url));

export default defineConfig({
  content: [
    join(here, "../web/pages/**/*.{html,tmpl}"),
    join(here, "./src/**/*.{js,ts}"),
  ],
  theme: {
    extend: {},
  },
  plugins: [daisyui()],
});
