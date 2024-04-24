import {EditorView, gutter, keymap, lineNumbers} from "@codemirror/view";
import {Compartment, EditorState, Facet, Line, SelectionRange} from "@codemirror/state";
import {indentLess} from "@codemirror/commands";

document.addEventListener("DOMContentLoaded", () => {
    EditorView.theme({}, {dark: true});

    let editorsjs: EditorView[] = [];
    let editorsParentdom = document.getElementById("editors")!;
    let allEditorsdom = document.querySelectorAll("#editors > .editor");
    let firstEditordom = allEditorsdom[0];

    const txtFacet = Facet.define<string>({
        combine(values) {
            return values;
        },
    });

    let indentSize = new Compartment(),
        wrapMode = new Compartment(),
        indentType = new Compartment();

    const newEditor = (dom: HTMLElement, value: string = ""): EditorView => {
        let editor = new EditorView({
            doc: value,
            parent: dom,
            extensions: [
                lineNumbers(),
                gutter({class: "cm-mygutter"}),
                keymap.of([{key: "Tab", run: customIndentMore, shift: indentLess}]),
                indentSize.of(EditorState.tabSize.of(2)),
                wrapMode.of([]),
                indentType.of(txtFacet.of("space")),
            ],
        });

        let mdpreview = dom.querySelector(".md-preview") as HTMLElement;

        let formfilename = dom.querySelector<HTMLInputElement>(".form-filename");

        // check if file ends with .md on pageload
        if (formfilename!.value.endsWith(".md")) {
            mdpreview!.classList.remove("hidden");
        } else {
            mdpreview!.classList.add("hidden");
        }

        // event if the filename ends with .md; trigger event
        formfilename!.onkeyup = (e) => {
            let filename = (e.target as HTMLInputElement).value;
            if (filename.endsWith(".md")) {
                mdpreview!.classList.remove("hidden");
            } else {
                mdpreview!.classList.add("hidden");
            }
        };

        // @ts-ignore
        const baseUrl = window.opengist_base_url || '';
        let previewShown = false;
        mdpreview.onclick = () => {
            previewShown = !previewShown;
            let divpreview = dom.querySelector("div.preview") as HTMLElement;
            let cmeditor = dom.querySelector(".cm-editor") as HTMLElement;

            if (!previewShown) {
                divpreview!.classList.add("hidden");
                cmeditor!.classList.remove("hidden-important");
                return;
            } else {
                fetch(`${baseUrl}/preview?` +  new URLSearchParams({
                    content: editor.state.doc.toString()
                }), {
                    method: 'GET',
                    credentials: 'same-origin',
                }).then(r => r.text()).then(r => {
                    let divpreview = dom.querySelector("div.preview") as HTMLElement;
                    divpreview!.innerHTML = r;
                    divpreview!.classList.remove("hidden");
                    cmeditor!.classList.add("hidden-important");
                })
            }
        }

        dom.querySelector<HTMLInputElement>(".editor-indent-type")!.onchange = (e) => {
            let newTabType = (e.target as HTMLInputElement).value;
            setIndentType(editor, !["tab", "space"].includes(newTabType) ? "space" : newTabType);
        };

        dom.querySelector<HTMLInputElement>(".editor-indent-size")!.onchange = (e) => {
            let newTabSize = parseInt((e.target as HTMLInputElement).value);
            setIndentSize(editor, ![2, 4, 8].includes(newTabSize) ? 2 : newTabSize);
        };

        dom.querySelector<HTMLInputElement>(".editor-wrap-mode")!.onchange = (e) => {
            let newWrapMode = (e.target as HTMLInputElement).value;
            setLineWrapping(editor, newWrapMode === "soft");
        };

        dom.addEventListener("drop", (e) => {
            e.preventDefault(); // prevent the browser from opening the dropped file
            (e.target as HTMLInputElement)
                .closest(".editor")
                .querySelector<HTMLInputElement>("input.form-filename")!.value =
                e.dataTransfer.files[0].name;
        });

        // remove editor on delete
        let deleteBtns = dom.querySelector<HTMLButtonElement>("button.delete-file");
        if (deleteBtns !== null) {
            deleteBtns.onclick = () => {
                editorsjs.splice(editorsjs.indexOf(editor), 1);
                dom.remove();
            };
        }

        editor.dom.addEventListener("input", function inputConfirmLeave() {
            if (!editor.inView) return; // skip events outside the viewport

            editor.dom.removeEventListener("input", inputConfirmLeave);
            window.onbeforeunload = () => {
                return "Are you sure you want to quit?";
            };
        });

        return editor;
    };

    function getIndentation(state: EditorState): string {
        // @ts-ignore
        if (indentType.get(state).value === "tab") {
            return "\t";
        }
        // @ts-ignore
        return " ".repeat(indentSize.get(state).value);
    }

    function customIndentMore({state, dispatch,}: { state: EditorState; dispatch: (value: any) => void; }): boolean {
        let indentation = getIndentation(state);
        dispatch({
            ...state.update(changeBySelectedLine(state, (line, changes) => {
                changes.push({from: state.selection.ranges[0].from, insert: indentation,});
            })),
            selection: {
                anchor: state.selection.ranges[0].from + indentation.length,
                head: state.selection.ranges[0].from + indentation.length,
            },
        });
        return true;
    }

    function changeBySelectedLine(state: EditorState, f: (line: Line, changes: any[]) => void): any {
        let atLine = -1;
        return state.changeByRange((range) => {
            let changes: any[] = [];
            for (let line = state.doc.lineAt(range.from); ;) {
                if (line.number > atLine) {
                    f(line, changes);
                    atLine = line.number;
                }
                if (range.to <= line.to) break;
                line = state.doc.lineAt(line.number + 1);
            }
            let changeSet = state.changes(changes);
            return {
                changes,
                // @ts-ignore
                range: new SelectionRange(changeSet.mapPos(range.anchor, 1), changeSet.mapPos(range.head, 1)),
            };
        });
    }

    function setIndentType(view: EditorView, type: string): void {
        view.dispatch({effects: indentType.reconfigure(txtFacet.of(type))});
    }

    function setIndentSize(view: EditorView, size: number): void {
        view.dispatch({effects: indentSize.reconfigure(EditorState.tabSize.of(size))});
    }

    function setLineWrapping(view: EditorView, enable: boolean): void {
        view.dispatch({
            effects: wrapMode.reconfigure(enable ? EditorView.lineWrapping : []),
        });
    }

    let arr = Array.from(allEditorsdom);
    arr.forEach((el: HTMLElement) => {
        // in case we edit the gist contents
        let currEditor = newEditor(el, el.querySelector<HTMLInputElement>(".form-filecontent")!.value);
        editorsjs.push(currEditor);
    });

    document.getElementById("add-file")!.onclick = () => {
        let newEditorDom = firstEditordom.cloneNode(true) as HTMLElement;

        // reset the filename of the new cloned element
        newEditorDom.querySelector<HTMLInputElement>('input[name="name"]')!.value = "";

        // removing the previous codemirror editor
        let newEditorDomCM = newEditorDom.querySelector(".cm-editor");
        newEditorDomCM!.remove();

        // creating the new codemirror editor and append it in the editor div
        editorsjs.push(newEditor(newEditorDom));
        editorsParentdom.append(newEditorDom);
    };

    document.querySelector<HTMLFormElement>("form#create")!.onsubmit = () => {
        let j = 0;
        document.querySelectorAll<HTMLInputElement>(".form-filecontent").forEach((e) => {
            e.value = encodeURIComponent(editorsjs[j++].state.doc.toString());
        });
    };

    document.getElementById('gist-metadata-btn')!.onclick = (el) => {
        let metadata = document.getElementById('gist-metadata')!;
        metadata.classList.toggle('hidden');

        let btn = el.target as HTMLButtonElement;
        if (btn.innerText.endsWith('▼')) {
            btn.innerText = btn.innerText.replace('▼', '▲');
        } else {
            btn.innerText = btn.innerText.replace('▲', '▼');
        }

    }

    document.onsubmit = () => {
        window.onbeforeunload = null;
    };
});
