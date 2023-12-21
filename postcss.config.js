module.exports = {
  plugins: {
    'postcss-import': {},
    'tailwindcss/nesting': {},
    tailwindcss: {},
    autoprefixer: {},
    'postcss-selector-namespace': {namespace() {return (process.env.EMBED) ? '.opengist-embed' : '';}},
    cssnano: {},
  },
}
