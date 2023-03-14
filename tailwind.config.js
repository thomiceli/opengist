const colors = require('tailwindcss/colors')

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
      emerald: colors.emerald,
      rose: colors.rose,
      primary: colors.sky,
      slate: colors.slate
    },
    extend: {
      borderWidth: {
        '1': '1px',
      }
    },
  },
  plugins: [require("@tailwindcss/typography"),require('@tailwindcss/forms')],
}
