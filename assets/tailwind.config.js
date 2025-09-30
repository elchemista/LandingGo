/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    "../web/pages/**/*.{html,tmpl}",
  ],
  theme: {
    extend: {
      colors: {
        brand: {
          DEFAULT: "#0a66c2",
          dark: "#155eab",
        },
      },
    },
  },
  plugins: [],
};
