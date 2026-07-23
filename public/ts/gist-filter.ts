// Faceted filter input for gist lists.
//
// A single box drives several backend query params. Free text is the title;
// qualifiers (`visibility:`, `language:`, `topic:`) chosen from the dropdown
// become code-styled chips. The chips + title are mirrored into the hidden
// title/visibility/language/topics inputs the server reads.

type Option = { value: string; label: string };

const KEYS = ['visibility', 'language', 'topic'] as const;
type Key = (typeof KEYS)[number];

interface Suggestion {
    label: string;
    qualifier?: Key; // completes "key:" in the input
    commit?: { key: Key; value: string }; // turns into a chip
}

function readOptions(form: HTMLElement, facet: string): Option[] {
    const tpl = form.querySelector<HTMLTemplateElement>(`template[data-filter-options="${facet}"]`);
    if (!tpl) return [];
    return Array.from(tpl.content.querySelectorAll<HTMLElement>('[data-value]')).map((el) => ({
        value: el.dataset.value || '',
        label: (el.textContent || '').trim(),
    }));
}

function setup(form: HTMLFormElement) {
    const box = form.querySelector<HTMLElement>('[data-filter-box]');
    const input = form.querySelector<HTMLInputElement>('[data-filter-input]');
    const menu = form.querySelector<HTMLElement>('[data-filter-menu]');
    if (!box || !input || !menu) return;

    const hidden = {
        title: form.querySelector<HTMLInputElement>('[data-filter-hidden="title"]'),
        visibility: form.querySelector<HTMLInputElement>('[data-filter-hidden="visibility"]'),
        language: form.querySelector<HTMLInputElement>('[data-filter-hidden="language"]'),
        topics: form.querySelector<HTMLInputElement>('[data-filter-hidden="topics"]'),
    };

    const options: Record<Key, Option[]> = {
        visibility: readOptions(form, 'visibility'),
        language: readOptions(form, 'language'),
        topic: [],
    };

    // Chosen qualifiers. visibility/language are single; topic can repeat.
    let tokens: { key: Key; value: string }[] = [];
    if (form.dataset.initVisibility) tokens.push({ key: 'visibility', value: form.dataset.initVisibility });
    if (form.dataset.initLanguage) tokens.push({ key: 'language', value: form.dataset.initLanguage });
    (form.dataset.initTopics || '')
        .split(/\s+/)
        .filter(Boolean)
        .forEach((v) => tokens.push({ key: 'topic', value: v }));
    input.value = form.dataset.initTitle || '';

    let suggestions: Suggestion[] = [];
    let active = -1;

    const lastToken = () => {
        const v = input.value;
        const sp = v.lastIndexOf(' ');
        return { token: v.slice(sp + 1), start: sp + 1 };
    };

    function commit(key: Key, value: string) {
        if (!value) return;
        if (key === 'topic') {
            if (!tokens.some((t) => t.key === 'topic' && t.value === value)) tokens.push({ key, value });
        } else {
            tokens = tokens.filter((t) => t.key !== key);
            tokens.push({ key, value });
        }
        renderChips();
    }

    function renderChips() {
        box.querySelectorAll('[data-chip]').forEach((n) => n.remove());
        for (const t of tokens) {
            const chip = document.createElement('span');
            chip.dataset.chip = '';
            chip.className =
                'bg-muted text-foreground inline-flex items-center gap-1 rounded border px-1.5 py-0.5 font-mono text-xs';
            const label = document.createElement('span');
            label.textContent = `${t.key}:${t.value}`;
            const rm = document.createElement('button');
            rm.type = 'button';
            rm.textContent = '×';
            rm.className = 'text-muted-foreground hover:text-foreground -mr-0.5 leading-none';
            rm.addEventListener('click', () => {
                tokens = tokens.filter((x) => x !== t);
                renderChips();
                syncHidden();
                input.focus();
            });
            chip.append(label, rm);
            box.insertBefore(chip, input);
        }
    }

    function build(): Suggestion[] {
        const { token } = lastToken();
        const colon = token.indexOf(':');
        if (colon === -1) {
            return KEYS.filter((k) => k.startsWith(token.toLowerCase())).map((k) => ({
                label: `${k}:`,
                qualifier: k,
            }));
        }
        const key = token.slice(0, colon).toLowerCase() as Key;
        const prefix = token.slice(colon + 1).toLowerCase();
        if (key === 'topic') {
            return prefix ? [{ label: `topic: ${prefix}`, commit: { key: 'topic', value: token.slice(colon + 1) } }] : [];
        }
        if (key === 'visibility' || key === 'language') {
            return options[key]
                .filter((o) => o.value.toLowerCase().includes(prefix))
                .slice(0, 30)
                .map((o) => ({ label: o.label, commit: { key, value: o.value } }));
        }
        return [];
    }

    function render() {
        suggestions = build();
        active = -1;
        if (suggestions.length === 0) {
            menu.classList.add('hidden');
            menu.innerHTML = '';
            return;
        }
        menu.innerHTML = '';
        suggestions.forEach((s, i) => {
            const btn = document.createElement('button');
            btn.type = 'button';
            btn.dataset.index = String(i);
            btn.textContent = s.label;
            btn.className =
                'flex w-full items-center rounded-sm px-2 py-1.5 text-left text-sm hover:bg-accent hover:text-accent-foreground aria-selected:bg-accent aria-selected:text-accent-foreground';
            menu.appendChild(btn);
        });
        menu.classList.remove('hidden');
    }

    function apply(s: Suggestion) {
        const { start } = lastToken();
        if (s.commit) {
            commit(s.commit.key, s.commit.value);
            input.value = input.value.slice(0, start); // drop the typed token; it's a chip now
        } else if (s.qualifier) {
            input.value = input.value.slice(0, start) + `${s.qualifier}:`;
        }
        input.focus();
        syncHidden();
        render();
    }

    function highlight(next: number) {
        const btns = Array.from(menu.querySelectorAll<HTMLButtonElement>('button'));
        if (btns.length === 0) return;
        active = (next + btns.length) % btns.length;
        btns.forEach((b, i) => b.setAttribute('aria-selected', i === active ? 'true' : 'false'));
        btns[active]?.scrollIntoView({ block: 'nearest' });
    }

    // Commit a fully-typed "key:value " token (space-terminated) into a chip.
    function commitOnSpace() {
        if (!input.value.endsWith(' ')) return;
        const trimmed = input.value.slice(0, -1);
        const sp = trimmed.lastIndexOf(' ');
        const last = trimmed.slice(sp + 1);
        const m = last.match(/^(visibility|language|topic):(.+)$/i);
        if (m) {
            commit(m[1].toLowerCase() as Key, m[2]);
            input.value = trimmed.slice(0, sp + 1);
        }
    }

    function syncHidden() {
        if (hidden.visibility) hidden.visibility.value = tokens.find((t) => t.key === 'visibility')?.value || '';
        if (hidden.language) hidden.language.value = tokens.find((t) => t.key === 'language')?.value || '';
        if (hidden.topics)
            hidden.topics.value = tokens
                .filter((t) => t.key === 'topic')
                .map((t) => t.value)
                .join(' ');
        // Title is the leftover free text, minus any half-typed qualifier token.
        if (hidden.title)
            hidden.title.value = input.value
                .split(/\s+/)
                .filter((w) => w && !/^(visibility|language|topic):/i.test(w))
                .join(' ');
    }

    input.addEventListener('focus', render);
    input.addEventListener('input', () => {
        commitOnSpace();
        syncHidden();
        render();
    });

    input.addEventListener('keydown', (e) => {
        const open = !menu.classList.contains('hidden');
        if (e.key === 'ArrowDown') {
            if (!open) render();
            highlight(active + 1);
            e.preventDefault();
        } else if (e.key === 'ArrowUp') {
            if (open) {
                highlight(active - 1);
                e.preventDefault();
            }
        } else if (e.key === 'Enter') {
            if (open && active >= 0 && suggestions[active]) {
                apply(suggestions[active]);
                e.preventDefault();
            }
        } else if (e.key === 'Escape') {
            menu.classList.add('hidden');
        } else if (e.key === 'Backspace' && input.value === '' && tokens.length > 0) {
            tokens.pop();
            renderChips();
            syncHidden();
        }
    });

    menu.addEventListener('mousedown', (e) => {
        const btn = (e.target as HTMLElement).closest<HTMLButtonElement>('button[data-index]');
        if (!btn) return;
        e.preventDefault(); // keep input focused
        const s = suggestions[Number(btn.dataset.index)];
        if (s) apply(s);
    });

    // Clicking anywhere in the box focuses the input.
    box.addEventListener('mousedown', (e) => {
        if (e.target === box) {
            input.focus();
            e.preventDefault();
        }
    });

    document.addEventListener('click', (e) => {
        if (!form.contains(e.target as Node)) menu.classList.add('hidden');
    });

    form.addEventListener('submit', syncHidden);

    renderChips();
    syncHidden();
}

export function initGistFilters() {
    document
        .querySelectorAll<HTMLFormElement>('form[data-gist-filter]:not([data-filter-ready])')
        .forEach((form) => {
            form.setAttribute('data-filter-ready', '1');
            setup(form);
        });
}
