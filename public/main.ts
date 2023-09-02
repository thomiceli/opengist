import './style.css';
import './style.scss';
import './favicon.svg';
import './default.png';
import moment from 'moment';
import md from 'markdown-it';
import hljs from 'highlight.js';


document.addEventListener('DOMContentLoaded', () => {
    const themeMenu = document.getElementById('theme-menu')!;

    document.getElementById('light-mode')!.onclick = (e) => {
        e.stopPropagation()
        localStorage.theme = 'light';
        themeMenu.classList.toggle('hidden');
        // @ts-ignore
        checkTheme()
    }

    document.getElementById('dark-mode')!.onclick = (e) => {
        e.stopPropagation()
        localStorage.theme = 'dark';
        themeMenu.classList.toggle('hidden');
        // @ts-ignore
        checkTheme()
    }

    document.getElementById('system-mode')!.onclick = (e) => {
        e.stopPropagation()
        localStorage.removeItem('theme');
        themeMenu.classList.toggle('hidden');
        // @ts-ignore
        checkTheme();
    }

    document.getElementById('theme-btn')!.onclick = (e) => {
        themeMenu.classList.toggle('hidden');
    }

    document.getElementById('user-btn')?.addEventListener("click" , (e) => {
        document.getElementById('user-menu').classList.toggle('hidden');
    })

    document.querySelectorAll('.moment-timestamp').forEach((e: HTMLElement) => {
        e.title = moment.unix(parseInt(e.innerHTML)).format('LLLL');
        e.innerHTML = moment.unix(parseInt(e.innerHTML)).fromNow();
    });

    document.querySelectorAll('.moment-timestamp-date').forEach((e: HTMLElement) => {
        e.innerHTML = moment.unix(parseInt(e.innerHTML)).format('DD/MM/YYYY HH:mm');
    });

    const rev = document.querySelector<HTMLElement>('.revision-text');
    if (rev) {
        const fullRev = rev.innerHTML;
        const smallRev = fullRev.substring(0, 7);
        rev.innerHTML = smallRev;

        rev.onmouseover = () => {
            rev.innerHTML = fullRev;
        };
        rev.onmouseout = () => {
            rev.innerHTML = smallRev;
        };
    }

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

    const colorhash = () => {
        Array.from(document.querySelectorAll('.table-code .selected')).forEach((el) => el.classList.remove('selected'));
        const lineEl = document.querySelector<HTMLElement>(location.hash);
        if (lineEl) {
            const nextSibling = lineEl.nextSibling;
            if (nextSibling instanceof HTMLElement) {
                nextSibling.classList.add('selected');
            }
        }
    };

    if (location.hash) {
        colorhash();
    }
    window.onhashchange = colorhash;

    document.getElementById('main-menu-button')!.onclick = () => {
        document.getElementById('mobile-menu')!.classList.toggle('hidden');
    };

    const tabs = document.getElementById('gist-tabs');
    if (tabs) {
        tabs.onchange = (e: Event) => {
            const target = e.target as HTMLSelectElement;
            window.location.href = target.selectedOptions[0].dataset.url || '';
        };
    }

    const gistmenutoggle = document.getElementById('gist-menu-toggle');
    if (gistmenutoggle) {
        const gistmenucopy = document.getElementById('gist-menu-copy')!;
        const gistmenubuttoncopy = document.getElementById('gist-menu-button-copy')!;
        const gistmenuinput = document.getElementById('gist-menu-input') as HTMLInputElement;
        const gistmenutitle = document.getElementById('gist-menu-title')!;

        gistmenutitle.textContent = gistmenucopy.children[0].firstChild!.textContent;
        gistmenuinput.value = (gistmenucopy.children[0] as HTMLElement).dataset.link || '';

        gistmenutoggle.onclick = () => {
            gistmenucopy.classList.toggle('hidden');
        };

        for (const item of Array.from(gistmenucopy.children)) {
            (item as HTMLElement).onclick = () => {
                gistmenutitle.textContent = item.firstChild!.textContent;
                gistmenuinput.value = (item as HTMLElement).dataset.link || '';
                gistmenucopy.classList.toggle('hidden');
            };
        }

        gistmenubuttoncopy.onclick = () => {
            const text = gistmenuinput.value;
            navigator.clipboard.writeText(text).catch((err) => {
                console.error('Could not copy text: ', err);
            });
        };
    }


    const sortgist = document.getElementById('sort-gists-button');
    if (sortgist) {
        sortgist.onclick = () => {
            document.getElementById('sort-gists-dropdown')!.classList.toggle('hidden');
        };
    }

    document.querySelectorAll('.copy-gist-btn').forEach((e: HTMLElement) => {
        e.onclick = () => {
            navigator.clipboard.writeText(e.parentNode!.parentNode!.querySelector<HTMLElement>('.gist-content')!.textContent || '').catch((err) => {
                console.error('Could not copy text: ', err);
            });
        };
    });

    const gistmenuvisibility = document.getElementById('gist-menu-visibility');
    if (gistmenuvisibility) {
        let submitgistbutton = (document.getElementById('submit-gist') as HTMLInputElement);
        document.getElementById('gist-visibility-menu-button')!.onclick = () => {
            console.log("z");
            gistmenuvisibility!.classList.toggle('hidden');
        }
        Array.from(document.querySelectorAll('.gist-visibility-option')).forEach((el) => {
            (el as HTMLElement).onclick = () => {
                submitgistbutton.textContent = "Create " + el.textContent.toLowerCase() + " gist";
                submitgistbutton!.value = (el as HTMLElement).dataset.visibility || '0';
                gistmenuvisibility!.classList.add('hidden');
            }
        });
    }
});
