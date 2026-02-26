/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    "./views/**/*.html",
    "./static/**/*.js"
  ],
  theme: {
    extend: {
      fontFamily: {
        sans: ["system-ui", "-apple-system", "BlinkMacSystemFont", "Segoe UI", "sans-serif"],
      },
      colors: {
        brand: {
          50: "#eef2ff",
          100: "#e0edff",
          500: "#6366f1",
          600: "#4f46e5"
        }
      },
      boxShadow: {
        "soft-xl": "0 24px 80px rgba(15,23,42,0.12)"
      }
    },
  },
  plugins: [],
};

