import './style.scss';
import './favicon-32.png';
import './opengist.svg';
import './default.png';
import dayjs from 'dayjs';
import relativeTime from 'dayjs/plugin/relativeTime';
import 'dayjs/locale/cs';
import 'dayjs/locale/de';
import 'dayjs/locale/es';
import 'dayjs/locale/fr';
import 'dayjs/locale/hu';
import 'dayjs/locale/pt';
import 'dayjs/locale/ru';
import 'dayjs/locale/zh';
import localizedFormat from 'dayjs/plugin/localizedFormat';

dayjs.extend(relativeTime);
dayjs.extend(localizedFormat);
dayjs.locale(window.opengist_locale || 'en');

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

    document.getElementById('theme-btn')!.onclick = () => {
        themeMenu.classList.toggle('hidden');
    }

    document.getElementById('user-btn')?.addEventListener("click" , () => {
        document.getElementById('user-menu').classList.toggle('hidden');
    })

    document.querySelectorAll('.moment-timestamp').forEach((e: HTMLElement) => {
        e.title = dayjs.unix(parseInt(e.innerHTML)).format('LLLL');
        e.innerHTML = dayjs.unix(parseInt(e.innerHTML)).fromNow();
    });

    document.querySelectorAll('.moment-timestamp-date').forEach((e: HTMLElement) => {
        e.innerHTML = dayjs.unix(parseInt(e.innerHTML)).format('DD/MM/YYYY HH:mm');
    });

    document.querySelectorAll('form').forEach((form: HTMLFormElement) => {
        form.onsubmit = () => {
            form.querySelectorAll('input[type=datetime-local]').forEach((input: HTMLInputElement) => {
                console.log(dayjs(input.value).unix());
                const hiddenInput = document.createElement('input');
                hiddenInput.type = 'hidden';
                hiddenInput.name = 'expiredAtUnix'
                hiddenInput.value = dayjs(input.value).unix().toString();
                form.appendChild(hiddenInput);
            });
            return true;
        };
    })



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

    document.getElementById('language-btn')!.onclick = () => {
        document.getElementById('language-list')!.classList.toggle('hidden');
    };


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
            gistmenuvisibility!.classList.toggle('hidden');
        }
        const lastVisibility = localStorage.getItem('visibility');
        Array.from(document.querySelectorAll('.gist-visibility-option')).forEach((el) => {
            const visibility = (el as HTMLElement).dataset.visibility || '0';
            (el as HTMLElement).onclick = () => {
                submitgistbutton.textContent = (el as HTMLElement).dataset.btntext;
                submitgistbutton!.value = visibility;
                localStorage.setItem('visibility', visibility);
                gistmenuvisibility!.classList.add('hidden');
            }
            if (lastVisibility === visibility) {
                (el as HTMLElement).click();
            }
        });
    }

    const searchinput = document.getElementById('search') as HTMLInputElement;
    searchinput.addEventListener('focusin', () => {
        document.getElementById('search-help').classList.remove('hidden');
    })

    searchinput.addEventListener('focusout', (e) => {
        document.getElementById('search-help').classList.add('hidden');
    })
});
