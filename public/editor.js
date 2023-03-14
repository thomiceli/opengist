import {EditorView, keymap, gutter, lineNumbers} from "@codemirror/view"
import {indentWithTab} from "@codemirror/commands"

EditorView.theme({}, {dark: true})

let editorsjs = []
let editorsParentdom = document.getElementById('editors')
let allEditorsdom = document.querySelectorAll('#editors > .editor')
let firstEditordom = allEditorsdom[0]

const newEditor = (dom, value = '') => {
    return new EditorView({
        doc: value,
        extensions: [
            lineNumbers(), gutter({class: "cm-mygutter"}),
            keymap.of([indentWithTab]),
        ],
        parent: dom
    })
}


document.onsubmit = () => {
    console.log('onsubmit');
    window.onbeforeunload = null;
}

let arr = [...allEditorsdom]
arr.forEach(el => {
    // in case we edit the gist contents
    let currEditor = newEditor(el, el.querySelector('.form-filecontent').value)
    editorsjs.push(currEditor)

    currEditor.dom.addEventListener("input", function inputConfirmLeave()  {
        if (!currEditor.inView) return; // skip events outside the viewport

        currEditor.dom.removeEventListener("input", inputConfirmLeave);
        window.onbeforeunload = () => {
            return 'Are you sure you want to quit?';
        }
    });

    // remove editor on delete
    let deleteBtns = el.querySelector('button.delete-file')
    if (deleteBtns !== null) {

        deleteBtns.onclick = () => {

            editorsjs.splice(editorsjs.indexOf(currEditor), 1);
            el.remove()
        }
    }
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
