/** @type {import('tailwindcss').Config} */
const path = require("path");

module.exports = {
  content: [
    path.join(__dirname, "../web/pages/**/*.{html,tmpl}"),
    path.join(__dirname, "./src/**/*.{js,ts}"),
  ],
  theme: {},
  plugins: [],
};
