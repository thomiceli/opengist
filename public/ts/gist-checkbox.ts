// Interactive markdown task-list checkboxes.
//
// The markdown renderer tags each task-list <li> with data-checkbox-nb="<n>"
// (see internal/render/markdown_checkbox.go). When the gist is owned by the
// logged-in user and not archived (#gist[data-own]), clicking a checkbox
// persists the toggle by PUTting to <gist-url>/checkbox, which rewrites and
// commits the file. Otherwise the checkboxes stay disabled and read-only.

function csrfToken(): string {
    const input = document.querySelector<HTMLInputElement>('input[name="_csrf"]');
    return input ? input.value : '';
}

function bindFile(article: HTMLElement) {
    const filename = article.dataset.file;
    if (!filename) return;

    const items = article.querySelectorAll<HTMLElement>('li[data-checkbox-nb]');
    items.forEach((item) => {
        const input = item.querySelector<HTMLInputElement>('input[type=checkbox]');
        if (!input || input.dataset.checkboxBound) return;
        input.dataset.checkboxBound = 'true';
        input.disabled = false;

        input.addEventListener('change', () => {
            const checkboxNb = item.dataset.checkboxNb;
            if (checkboxNb === undefined) return;

            // Disable every checkbox on the page while the write is in flight so
            // concurrent toggles can't race the file rewrite on the server.
            const all = document.querySelectorAll<HTMLInputElement>('li[data-checkbox-nb] input[type=checkbox]');
            all.forEach((el) => (el.disabled = true));

            const data = new URLSearchParams();
            data.append('checkbox', checkboxNb);
            data.append('file', filename);
            const csrf = csrfToken();
            if (csrf) data.append('_csrf', csrf);

            fetch(location.href.split('#')[0].split('?')[0] + '/checkbox', {
                method: 'PUT',
                credentials: 'same-origin',
                body: data,
            })
                .then((response) => {
                    if (!response.ok) input.checked = !input.checked; // revert on failure
                })
                .catch(() => {
                    input.checked = !input.checked;
                })
                .finally(() => {
                    all.forEach((el) => (el.disabled = false));
                });
        });
    });
}

export function initGistCheckboxes() {
    const gist = document.getElementById('gist');
    const editable = !!gist && gist.dataset.own === 'true';

    document.querySelectorAll<HTMLElement>('article[data-file]').forEach((article) => {
        if (editable) {
            bindFile(article);
        } else {
            article
                .querySelectorAll<HTMLInputElement>('li[data-checkbox-nb] input[type=checkbox]')
                .forEach((input) => (input.disabled = true));
        }
    });
}
