import {EditorView, gutter, keymap, lineNumbers} from "@codemirror/view"
import {Compartment, EditorState, Facet, SelectionRange} from "@codemirror/state"
import {indentLess} from "@codemirror/commands";

document.addEventListener('DOMContentLoaded', () => {
    EditorView.theme({}, {dark: true})

    let editorsjs = []
    let editorsParentdom = document.getElementById('editors')
    let allEditorsdom = document.querySelectorAll('#editors > .editor')
    let firstEditordom = allEditorsdom[0]

    const txtFacet = Facet.define({
        combine(values) {
            return values[0]
        }
    })
    let indentSize = new Compartment, wrapMode = new Compartment, indentType = new Compartment

    const newEditor = (dom, value = '') => {
        let editor = new EditorView({
            doc: value,
            parent: dom,
            extensions: [
                lineNumbers(), gutter({class: "cm-mygutter"}),
                keymap.of([{key: "Tab", run: customIndentMore, shift: indentLess}]),
                indentSize.of(EditorState.tabSize.of(2)),
                wrapMode.of([]),
                indentType.of(txtFacet.of("space")),
            ]
        })

        dom.querySelector('.editor-indent-type').onchange = (e) => {
            let newTabType = e.target.value
            setIndentType(editor, !['tab', 'space'].includes(newTabType) ? 'space' : newTabType)
        }

        dom.querySelector('.editor-indent-size').onchange = (e) => {
            let newTabSize = parseInt(e.target.value)
            setIndentSize(editor, ![2, 4, 8].includes(newTabSize) ? 2 : newTabSize)
        }

        dom.querySelector('.editor-wrap-mode').onchange = (e) => {
            let newWrapMode = e.target.value
            setLineWrapping(editor, newWrapMode === 'soft')
        }

        dom.addEventListener("drop", (e) => {
            e.preventDefault(); // prevent the browser from opening the dropped file
            e.target.closest('.editor').querySelector('input.form-filename').value = e.dataTransfer.files[0].name
        });

        // remove editor on delete
        let deleteBtns = dom.querySelector('button.delete-file')
        if (deleteBtns !== null) {
            deleteBtns.onclick = () => {
                editorsjs.splice(editorsjs.indexOf(editor), 1);
                dom.remove()
            }
        }

        editor.dom.addEventListener("input", function inputConfirmLeave() {
            if (!editor.inView) return; // skip events outside the viewport

            editor.dom.removeEventListener("input", inputConfirmLeave);
            window.onbeforeunload = () => {
                return 'Are you sure you want to quit?';
            }
        });

        return editor;
    }

    function getIndentation(state) {
        if (indentType.get(state).value === 'tab') {
            return '\t';
        }
        return ' '.repeat(indentSize.get(state).value);
    }

    function customIndentMore({state, dispatch}) {
        let indentation = getIndentation(state)
        dispatch({
            ...state.update(changeBySelectedLine(state, (line, changes) => {
                changes.push({from: state.selection.ranges[0].from, insert: indentation})
            })), selection: {
                anchor: state.selection.ranges[0].from + indentation.length,
                head: state.selection.ranges[0].from + indentation.length,
            }
        })
        return true
    }

    function changeBySelectedLine(state, f) {
        let atLine = -1
        return state.changeByRange(range => {
            let changes = []
            for (let line = state.doc.lineAt(range.from); ;) {
                if (line.number > atLine) {
                    f(line, changes)
                    atLine = line.number
                }
                if (range.to <= line.to) break
                line = state.doc.lineAt(line.number + 1)
            }
            let changeSet = state.changes(changes)
            return {
                changes,
                range: new SelectionRange(changeSet.mapPos(range.anchor, 1), changeSet.mapPos(range.head, 1))
            }
        })
    }

    function setIndentType(view, type) {
        view.dispatch({effects: indentType.reconfigure(txtFacet.of(type))})
    }

    function setIndentSize(view, size) {
        view.dispatch({effects: indentSize.reconfigure(EditorState.tabSize.of(size))})
    }

    function setLineWrapping(view, enable) {
        if (enable) {
            view.dispatch({effects: wrapMode.reconfigure(EditorView.lineWrapping)})
        } else {
            view.dispatch({effects: wrapMode.reconfigure([])})
        }
    }

    let arr = [...allEditorsdom]
    arr.forEach(el => {
        // in case we edit the gist contents
        let currEditor = newEditor(el, el.querySelector('.form-filecontent').value)
        editorsjs.push(currEditor)
    })

    document.getElementById('add-file').onclick = () => {
        let newEditorDom = firstEditordom.cloneNode(true)

        // reset the filename of the new cloned element
        newEditorDom.querySelector('input[name="name"]').value = ""

        // removing the previous codemirror editor
        let newEditorDomCM = newEditorDom.querySelector('.cm-editor')
        newEditorDomCM.remove()

        // creating the new codemirror editor and append it in the editor div
        editorsjs.push(newEditor(newEditorDom))
        editorsParentdom.append(newEditorDom)
    }

    document.querySelector('form#create').onsubmit = () => {
        let j = 0
        document.querySelectorAll('.form-filecontent').forEach((e) => {
            e.value = encodeURIComponent(editorsjs[j++].state.doc.toString())
        })
    }

    document.onsubmit = () => {
        window.onbeforeunload = null;
    }
})