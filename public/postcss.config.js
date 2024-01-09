module.exports = {
  plugins: {
    'postcss-import': {},
    'tailwindcss/nesting': {},
    tailwindcss: {
      config: "./public/tailwind.config.js",
    },
    autoprefixer: {},
    'postcss-selector-namespace': {namespace() {return (process.env.EMBED) ? '.opengist-embed' : '';}},
    cssnano: {},
  },
}
