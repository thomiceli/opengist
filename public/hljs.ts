import hljs from 'highlight.js';
import md from 'markdown-it';

document.querySelectorAll('.markdown').forEach((e: HTMLElement) => {
    e.innerHTML = md({
        html: true,
        highlight: function (str, lang) {
            if (lang && hljs.getLanguage(lang)) {
                try {
                    return '<pre class="hljs"><code>' +
                        hljs.highlight(str, {language: lang, ignoreIllegals: true}).value +
                        '</code></pre>';
                } catch (__) {
                }
            }

            return '<pre class="hljs"><code>' + md().utils.escapeHtml(str) + '</code></pre>';
        }
    }).render(e.textContent);
});

document.querySelectorAll<HTMLElement>('.table-code').forEach((el) => {
    const ext = el.dataset.filename?.split('.').pop() || '';

    if (hljs.autoDetection(ext) && ext !== 'txt') {
        el.querySelectorAll<HTMLElement>('td.line-code').forEach((ell) => {
            ell.classList.add('language-' + ext);
            hljs.highlightElement(ell);
        });
    }

    el.addEventListener('click', event => {
        if (event.target && (event.target as HTMLElement).matches('.line-num')) {
            Array.from(document.querySelectorAll('.table-code .selected')).forEach((el) => el.classList.remove('selected'));

            const nextSibling = (event.target as HTMLElement).nextSibling;
            if (nextSibling instanceof HTMLElement) {
                nextSibling.classList.add('selected');
            }


            const filename = el.dataset.filenameSlug;
            const line = (event.target as HTMLElement).textContent;
            const url = location.protocol + '//' + location.host + location.pathname;
            const hash = '#file-' + filename + '-' + line;
            window.history.pushState(null, null, url + hash);
            location.hash = hash;
        }
    });
});
