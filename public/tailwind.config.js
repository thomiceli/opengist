const colors = require('tailwindcss/colors')

/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    "./templates/**/*.html",
  ],
  theme: {
    colors: {
      transparent: 'transparent',
      current: 'currentColor',
      white: colors.white,
      black: colors.black,
      yellow: colors.yellow,
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
      rose: colors.rose,
      primary: {
        50: '#d6e1ff',
        100: '#d1dfff',
        200: '#b9d2fe',
        300: '#84b1fb',
        400: '#74a4f6',
        500: '#588fee',
        600: '#3c79e2',
        700: '#356fc0',
        800: '#2b5da3',
        900: '#1f4b8c',
        950: '#192b57'
      },

      slate: colors.slate
    },
    extend: {
      borderWidth: {
        '1': '1px',
      }
    },
  },
  plugins: [require("@tailwindcss/typography"),require('@tailwindcss/forms')],
  darkMode: 'class',
}
