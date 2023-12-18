import md from 'markdown-it';

document.querySelectorAll('.markdown').forEach((e: HTMLElement) => {
    e.innerHTML = md({
        html: true,

    }).render(e.textContent);
});

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
