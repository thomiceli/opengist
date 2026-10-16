// Delegation also covers editor previews and content swapped in by htmx.
document.addEventListener('click', async (event) => {
    if (!(event.target instanceof Element)) return;
    const button = event.target.closest<HTMLButtonElement>('.copy-code-btn');
    if (!button) return;

    const code = button.closest('.code-block-wrapper')?.querySelector('pre code');
    if (!code) return;

    const content = code.cloneNode(true) as HTMLElement;
    content.querySelectorAll('.ln, .lnt').forEach((lineNumber) => lineNumber.remove());

    try {
        await navigator.clipboard.writeText(content.textContent || '');
        button.dataset.copied = 'true';
        setTimeout(() => delete button.dataset.copied, 1200);
    } catch (err) {
        console.error('Could not copy code: ', err);
    }
});
