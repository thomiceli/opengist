document.addEventListener('DOMContentLoaded', () => {
    registerDomSetting(document.getElementById('disable-signup') as HTMLInputElement);
});

const setSetting = (key: string, value: string) => {
    const data = new URLSearchParams();
    data.append('key', key);
    data.append('value', value);
    data.append('_csrf', ((document.getElementsByName('_csrf')[0] as HTMLInputElement).value));
    fetch('/admin-panel/set-setting', {
        method: 'PUT',
        credentials: 'same-origin',
        body: data,
    });
};

const registerDomSetting = (el: HTMLInputElement) => {
    el.addEventListener('change', () => {
        setSetting(el.id, el.checked ? '1' : '0');
    });
};

