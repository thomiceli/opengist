// Line selection + permalinks for gist file views.
//
// Each line-number cell has id="file-<slug>-<n>". Clicking a number highlights
// that line and writes the hash #file-<slug>-<n> to the URL. Loading or
// navigating to such a hash scrolls to and highlights the line.

function clearSelection() {
    document.querySelectorAll('.table-code .selected').forEach((el) => el.classList.remove('selected'));
}

function select(numCell: HTMLElement) {
    clearSelection();
    const code = numCell.nextElementSibling;
    if (code) code.classList.add('selected');
}

function highlightFromHash() {
    if (!location.hash.startsWith('#file-')) return;
    const numCell = document.getElementById(location.hash.slice(1));
    if (!numCell || !numCell.classList.contains('line-num')) return;
    select(numCell);
    numCell.scrollIntoView({ block: 'center' });
}

export function initGistLines() {
    highlightFromHash();

    const w = window as unknown as { __gistLinesBound?: boolean };
    if (w.__gistLinesBound) return; // delegated handlers are global; bind once
    w.__gistLinesBound = true;

    document.addEventListener('click', (e) => {
        const numCell = (e.target as HTMLElement).closest<HTMLElement>('.table-code td.line-num[id]');
        if (!numCell) return;
        select(numCell);
        history.replaceState(null, '', location.pathname + location.search + '#' + numCell.id);
    });

    window.addEventListener('hashchange', highlightFromHash);
}
