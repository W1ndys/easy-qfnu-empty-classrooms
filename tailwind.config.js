/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ["./web/**/*.{html,js}"],
  theme: {
    extend: {
      colors: {
        // We are using CSS variables in input.css for colors,
        // but if we wanted to use them as utilities we could define them here.
        // For now, the existing arbitrary value usage in HTML (e.g. text-[#885021]) works fine with JIT.
      },
    },
  },
  plugins: [],
}
