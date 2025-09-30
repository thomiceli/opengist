import '../css/tailwind.css';
import '../img/favicon-32.png';
import '../img/opengist.svg';
import jdenticon from 'jdenticon/standalone';

jdenticon.update("[data-jdenticon-value]")

document.addEventListener('DOMContentLoaded', () => {
    document.getElementById('user-btn')?.addEventListener("click" , () => {
        document.getElementById('user-menu')!.classList.toggle('hidden');
    })

    document.querySelectorAll('form').forEach((form: HTMLFormElement) => {
        form.onsubmit = () => {
            form.querySelectorAll('input[type=datetime-local]').forEach((input: HTMLInputElement) => {
                const hiddenInput = document.createElement('input');
                hiddenInput.type = 'hidden';
                hiddenInput.name = 'expiredAtUnix'
                hiddenInput.value = Math.floor(new Date(input.value).getTime() / 1000).toString();
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

    const searchUserGistsVisibility = document.getElementById('search-user-gists-visibility');
    if (searchUserGistsVisibility) {
        let dropdown = document.getElementById('search-user-gists-visibility-dropdown');
        searchUserGistsVisibility.onclick = () => {
            dropdown!.classList.toggle('hidden');
        };

        let buttons = dropdown.querySelectorAll('button');
        buttons.forEach((button) => {
            button.onclick = () => {
                let value = document.getElementById('visibility-value') as HTMLInputElement;
                value.textContent = button.dataset.visibilityStr;
                dropdown!.classList.add('hidden');
                dropdown.querySelector('input')!.value = button.dataset.visibility || '';
            };
        });
    }

    const searchUserGistsLanguage = document.getElementById('search-user-gists-language');
    if (searchUserGistsLanguage) {
        let dropdown = document.getElementById('search-user-gists-language-dropdown');
        searchUserGistsLanguage.onclick = () => {
            dropdown!.classList.toggle('hidden');
        };
        let buttons = dropdown.querySelectorAll('button');
        buttons.forEach((button) => {
            button.onclick = () => {
                let value = document.getElementById('language-value') as HTMLInputElement;
                value.textContent = button.dataset.languageStr;
                dropdown!.classList.add('hidden');
                dropdown.querySelector('input')!.value = button.dataset.language || '';
            };
        });
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
