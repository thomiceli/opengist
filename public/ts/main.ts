import '../css/main.css';
import '../img/favicon-32.png';
import '../img/opengist.svg';

import 'htmx.org';
import 'hyperscript.org';
import 'basecoat-css/basecoat';
import 'basecoat-css/dropdown-menu';
import { initGistFilters } from './gist-filter';
import { initGistLines } from './gist-lines';
import { initJdenticon } from './jdenticon';
import { initIpynb } from './ipynb';
import { initPdf } from './pdf';

const init = () => {
    initGistFilters();
    initGistLines();
    initIpynb();
    initPdf();
    initJdenticon();
};

document.addEventListener('DOMContentLoaded', init);
// Re-init after hx-boost / htmx content swaps.
document.body.addEventListener('htmx:afterSwap', init);

// Top loading bar for boosted navigations. Large files can take a moment to
// render server-side, and htmx gives no visual feedback in the meantime, so the
// page looks frozen after a click. This animates a slim bar at the top of the
// viewport while the request is in flight.
//
// hx-boost swaps the whole <body> innerHTML on navigation, which would wipe any
// element we place in the template. We own the bar in JS instead and re-attach
// it after every swap so the reference never goes stale.
(() => {
    const bar = document.createElement('div');
    bar.id = 'page-progress';
    bar.setAttribute('aria-hidden', 'true');

    let trickle: number | undefined;
    let showTimer: number | undefined;
    let progress = 0;

    const set = (value: number) => {
        progress = value;
        bar.style.setProperty('--progress', String(value));
    };

    const attach = () => {
        // A history snapshot may have restored a stale copy of the bar (see the
        // beforeHistorySave handler below). Drop any that isn't ours.
        document.querySelectorAll('#page-progress').forEach((el) => {
            if (el !== bar) el.remove();
        });
        if (bar.parentElement !== document.body) document.body.appendChild(bar);
    };
    attach();
    document.body.addEventListener('htmx:afterSwap', attach);

    // The bar is chrome, not page content, so keep it out of the DOM snapshot
    // htmx saves for back/forward navigation — otherwise a bar frozen mid-load
    // gets restored and sticks at the top. Detaching + clearing state leaves the
    // snapshot clean; attach() re-adds a fresh bar on the way back.
    document.body.addEventListener('htmx:beforeHistorySave', () => {
        window.clearTimeout(showTimer);
        window.clearInterval(trickle);
        bar.classList.remove('is-loading');
        set(0);
        bar.remove();
    });

    const start = () => {
        window.clearTimeout(showTimer);
        window.clearInterval(trickle);
        // Delay showing so quick navigations don't flash the bar.
        showTimer = window.setTimeout(() => {
            attach();
            bar.classList.add('is-loading');
            set(0.08);
            // Creep toward (but never reach) the end while we wait.
            trickle = window.setInterval(() => {
                if (progress < 0.9) set(progress + (0.9 - progress) * 0.1);
            }, 300);
        }, 150);
    };

    const done = () => {
        window.clearTimeout(showTimer);
        window.clearInterval(trickle);
        if (!bar.classList.contains('is-loading')) return;
        // Fill to the end, then fade out. Reset the width only once it's fully
        // invisible so the bar never appears to slide backwards.
        attach();
        set(1);
        window.setTimeout(() => bar.classList.remove('is-loading'), 200);
        window.setTimeout(() => {
            bar.style.transition = 'none';
            set(0);
            void bar.offsetWidth; // flush before restoring transitions
            bar.style.transition = '';
        }, 500);
    };

    document.body.addEventListener('htmx:beforeRequest', start);
    document.body.addEventListener('htmx:afterRequest', done);
    document.body.addEventListener('htmx:historyRestore', () => {
        attach();
        done();
    });
})();

// htmx ignores error responses (4xx/5xx) by default, so a boosted navigation to
// a page that errors (e.g. a 404) would leave the user on the old page. Tell htmx
// to swap the error page in anyway so it renders like a normal navigation.
document.body.addEventListener('htmx:beforeSwap', (e) => {
    const detail = (e as CustomEvent).detail as { xhr: XMLHttpRequest; shouldSwap: boolean; isError: boolean };
    if (detail.xhr && detail.xhr.status >= 400) {
        detail.shouldSwap = true;
        detail.isError = false;
    }
});

// hx-boost stores a DOM snapshot for the back/forward cache. Components mark
// themselves initialized (basecoat: data-*-initialized, our filter:
// data-filter-ready), but the restored snapshot keeps those markers while the
// JS listeners are gone — so dropdowns etc. look dead. Strip the markers before
// the snapshot is saved, and re-initialize on restore.
const INIT_MARKERS = '[data-dropdown-menu-initialized], [data-filter-ready]';
const stripInitMarkers = () => {
    document.querySelectorAll(INIT_MARKERS).forEach((el) => {
        el.removeAttribute('data-dropdown-menu-initialized');
        el.removeAttribute('data-filter-ready');
    });
};

document.body.addEventListener('htmx:beforeHistorySave', stripInitMarkers);
document.body.addEventListener('htmx:historyRestore', () => {
    stripInitMarkers();
    (window as unknown as { basecoat?: { initAll?: () => void } }).basecoat?.initAll?.();
    init();
});
