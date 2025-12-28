document.addEventListener('DOMContentLoaded', () => {
    const noSoftWrapRadio = document.getElementById('no-soft-wrap');
    const softWrapRadio = document.getElementById('soft-wrap');

    function updateRootClass() {
        const table = document.querySelector("table");

        if (softWrapRadio.checked) {
            table.classList.remove('whitespace-pre');
            table.classList.add('whitespace-pre-wrap');
        } else {
            table.classList.remove('whitespace-pre-wrap');
            table.classList.add('whitespace-pre');
        }
    }

    noSoftWrapRadio.addEventListener('change', updateRootClass);
    softWrapRadio.addEventListener('change', updateRootClass);


    document.getElementById('removedlinecolor').addEventListener('change', function(event) {
        const color = hexToRgba(event.target.value, 0.1);
        document.documentElement.style.setProperty('--red-diff', color);
    });

    document.getElementById('addedlinecolor').addEventListener('change', function(event) {
        const color = hexToRgba(event.target.value, 0.1);
        document.documentElement.style.setProperty('--green-diff', color);
    });

    document.getElementById('gitlinecolor').addEventListener('change', function(event) {
        const color = hexToRgba(event.target.value, 0.38);
        document.documentElement.style.setProperty('--git-diff', color);
    });
});

function hexToRgba(hex, opacity) {
    hex = hex.replace('#', '');

    const r = parseInt(hex.substring(0, 2), 16);
    const g = parseInt(hex.substring(2, 4), 16);
    const b = parseInt(hex.substring(4, 6), 16);

    return `rgba(${r}, ${g}, ${b}, ${opacity})`;
}