const colors = require('tailwindcss/colors')

/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    "./.vitepress/theme/*.vue",
  ],
  theme: {
    colors: {
      transparent: 'transparent',
      current: 'currentColor',
      white: colors.white,
      black: colors.black,
      gray: {
        50: "#EEEFF1",
        100: "#DEDFE3",
        200: "#BABCC5",
        300: "#999CA8",
        400: "#75798A",
        500: "#585B68",
        600: "#464853",
        700: "#363840",
        800: "#232429",
        900: "#131316"
      },
      indigo: colors.indigo,

    },
    extend: {
      borderWidth: {
        '1': '1px',
      }
    },
  },
  plugins: [],
  darkMode: 'class',
}
