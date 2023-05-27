document.addEventListener('DOMContentLoaded', () => {
    let elems = Array.from(document.getElementsByClassName("toggle-button"));
    for (let elem of elems) {
        elem.addEventListener('click', () => {
            registerDomSetting(elem as HTMLElement)
        })
    }
});

const setSetting = (key: string, value: string) => {
    const data = new URLSearchParams();
    data.append('key', key);
    data.append('value', value);
    data.append('_csrf', ((document.getElementsByName('_csrf')[0] as HTMLInputElement).value));
    return fetch('/admin-panel/set-setting', {
        method: 'PUT',
        credentials: 'same-origin',
        body: data,
    });
};

const registerDomSetting = (el: HTMLElement) => {
    // @ts-ignore
    el.dataset["bool"] = !(el.dataset["bool"] === 'true');
    setSetting(el.id, el.dataset["bool"] === 'true' ? '1' : '0')
        .then(() => {
            el.classList.toggle("bg-primary-600");
            el.classList.toggle("dark:bg-gray-400");
            el.classList.toggle("bg-gray-300");
            (el.childNodes.item(1) as HTMLElement).classList.toggle("translate-x-5");
        });
};

