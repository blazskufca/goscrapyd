/** @type {import('tailwindcss').Config} */
module.exports = {
  darkMode: 'class',
  content: [
      "./assets/templates/**/*.tmpl",
      "./assets/templates/errors/*.tmpl",
      "./assets/templates/base/*.tmpl",
      "./assets/static/js/*.js"
  ],
  theme: {
    extend: {},
  },
  plugins: [],
}
