import katex from 'katex';

const delimiters = [
  { left: '\\\$\\\$', right: '\\\$\\\$', multiline: true },
  { left: '\\\$', right: '\\\$', multiline: false },
  { left: '\\\\\[', right: '\\\\\]', multiline: true },
  { left: '\\\\\(', right: '\\\\\)', multiline: false },
];

const delimiterMatchers = delimiters.map(
  (delimiter) => new RegExp(
    `${delimiter.left}(.*?)${delimiter.right}`,
    `g${delimiter.multiline ? 'ms' : ''}`
  )
);

// Replace LaTeX delimiters in a string with KaTeX rendering
function render(text: string): string {
  // Step 1: Replace all LaTeX expressions with placeholders
  const expressions: Array<{ placeholder: string; latex: string; displayMode: boolean }> = [];
  let modifiedText = text;
  let placeholderIndex = 0;

  // Process each delimiter type
  delimiters.forEach((delimiter, i) => {
    // Find all matches and replace with placeholders
    modifiedText = modifiedText.replace(delimiterMatchers[i], (match, latex) => {
      if (!latex.trim()) {
        return match; // Return original if content is empty
      }

      const placeholder = `__KATEX_PLACEHOLDER_${placeholderIndex++}__`;
      expressions.push({
        placeholder,
        latex,
        displayMode: delimiter.multiline,
      });

      return placeholder;
    });
  });

  // Step 2: Replace placeholders with rendered LaTeX
  for (const { placeholder, latex, displayMode } of expressions) {
    try {
      const rendered = katex.renderToString(latex, {
        throwOnError: false,
        displayMode,
      });
      modifiedText = modifiedText.replace(placeholder, rendered);
    } catch (error) {
      console.error('KaTeX rendering error:', error);
      // Replace placeholder with original LaTeX if rendering fails
      modifiedText = modifiedText.replace(
        placeholder,
        displayMode ? `$$${latex}$$` : `$${latex}$`
      );
    }
  }

  return modifiedText;
}

export default {
  render,
};
