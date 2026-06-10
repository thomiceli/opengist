import "../css/embed.css"

document.addEventListener('DOMContentLoaded', () => {
    document.querySelectorAll<HTMLElement>('.copy-embed-btn').forEach((btn) => {
        btn.addEventListener('click', () => {
            const content = btn.closest('.rounded-md')?.querySelector<HTMLElement>('.gist-content')?.textContent || '';
            navigator.clipboard.writeText(content).catch((err) => {
                console.error('Could not copy text: ', err);
            });
        });
    });
});
