document.querySelectorAll<HTMLElement>('.table-code').forEach((el) => {
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

let copybtnhtml = `<button type="button" style="top: 1em !important; right: 1em !important;" class="md-code-copy-btn absolute focus-within:z-auto rounded-md dark:border-gray-600 px-2 py-2 opacity-80 font-medium text-slate-700 bg-gray-100 dark:bg-gray-700 dark:text-slate-300 hover:bg-gray-200 dark:hover:bg-gray-600 hover:border-gray-500 hover:text-slate-700 dark:hover:text-slate-300 focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500"><svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="w-5 h-5"><path stroke-linecap="round" stroke-linejoin="round" d="M8.25 7.5V6.108c0-1.135.845-2.098 1.976-2.192.373-.03.748-.057 1.123-.08M15.75 18H18a2.25 2.25 0 002.25-2.25V6.108c0-1.135-.845-2.098-1.976-2.192a48.424 48.424 0 00-1.123-.08M15.75 18.75v-1.875a3.375 3.375 0 00-3.375-3.375h-1.5a1.125 1.125 0 01-1.125-1.125v-1.5A3.375 3.375 0 006.375 7.5H5.25m11.9-3.664A2.251 2.251 0 0015 2.25h-1.5a2.251 2.251 0 00-2.15 1.586m5.8 0c.065.21.1.433.1.664v.75h-6V4.5c0-.231.035-.454.1-.664M6.75 7.5H4.875c-.621 0-1.125.504-1.125 1.125v12c0 .621.504 1.125 1.125 1.125h9.75c.621 0 1.125-.504 1.125-1.125V16.5a9 9 0 00-9-9z" /></svg></button>`;

document.querySelectorAll<HTMLElement>('.markdown-body pre').forEach((el) => {
    if (el.classList.contains("mermaid")) {
        return;
    }
    el.innerHTML = copybtnhtml + `<span class="code-div">` + el.innerHTML + `</span>`;
});

document.querySelectorAll('.md-code-copy-btn').forEach(button => {
    button.addEventListener('click', function() {
        let code = this.nextElementSibling.textContent;
        navigator.clipboard.writeText(code).catch((err) => {
            console.error('Could not copy text: ', err);
        });
    });
});

let checkboxes = document.querySelectorAll('li[data-checkbox-nb] input[type=checkbox]');
if (document.getElementById('gist').dataset.own) {
    document.querySelectorAll<HTMLElement>('li[data-checkbox-nb]').forEach((el) => {
        let input: HTMLButtonElement = el.querySelector('input[type=checkbox]');
        input.disabled = false;
        let checkboxNb = (el as HTMLElement).dataset.checkboxNb;
        let filename = input.closest<HTMLElement>('div[data-file]').dataset.file;

        input.addEventListener('change', function () {
            const data = new URLSearchParams();
            data.append('checkbox', checkboxNb);
            data.append('file', filename);
            if (document.getElementsByName('_csrf').length !== 0) {
                data.append('_csrf', ((document.getElementsByName('_csrf')[0] as HTMLInputElement).value));
            }
            checkboxes.forEach((el: HTMLButtonElement) => {
                el.disabled = true;
                el.classList.add('text-gray-400')
            });
            fetch(window.location.href.split('#')[0] + '/checkbox', {
                method: 'PUT',
                credentials: 'same-origin',
                body: data,
            }).then((response) => {
                if (response.status === 200) {
                    checkboxes.forEach((el: HTMLButtonElement) => {
                        el.disabled = false;
                        el.classList.remove('text-gray-400')
                    });
                }
            });
        });
    });
} else {
    checkboxes.forEach((el: HTMLButtonElement) => {
        el.disabled = true;
    });
}



