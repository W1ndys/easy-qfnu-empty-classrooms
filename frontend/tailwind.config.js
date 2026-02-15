/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{vue,js,ts,jsx,tsx}'],
  theme: {
    extend: {
      colors: {
        primary: {
          DEFAULT: '#885021',
          light: '#8B5A2B',
          lighter: '#A67C52',
          dark: '#5D3615',
        },
      },
    },
  },
  plugins: [],
}
