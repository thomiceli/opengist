import {EditorView, gutter, keymap, lineNumbers} from "@codemirror/view";
import {Compartment, EditorState, Facet, Line, SelectionRange} from "@codemirror/state";
import {defaultKeymap, indentLess} from "@codemirror/commands";

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
                keymap.of([
                    {key: "Tab", run: customIndentMore, shift: indentLess},
                    ...defaultKeymap,
                ]),
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
                const formData = new FormData();
                formData.append('content', editor.state.doc.toString());
                let csrf = document.querySelector<HTMLInputElement>('form#create input[name="_csrf"]').value
                fetch(`${baseUrl}/preview`, {
                    method: 'POST',
                    credentials: 'same-origin',
                    body: formData,
                    headers: {
                        'X-CSRF-Token': csrf
                    }
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
                // For both text and binary files, just remove from DOM
                if (!dom.hasAttribute('data-binary-original-name')) {
                    // Only remove from editors array for text files
                    editorsjs.splice(editorsjs.indexOf(editor), 1);
                }
                dom.remove();
                checkForFirstDeleteButton();
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
        let formFileContent =el.querySelector<HTMLInputElement>(".form-filecontent")
        if (formFileContent !== null) {
            let currEditor = newEditor(el, el.querySelector<HTMLInputElement>(".form-filecontent")!.value);
            editorsjs.push(currEditor);
        } else if (el.hasAttribute('data-binary-original-name')) {
            // For binary files, just set up the delete button
            let deleteBtn = el.querySelector<HTMLButtonElement>("button.delete-file");
            if (deleteBtn) {
                deleteBtn.onclick = () => {
                    el.remove();
                    checkForFirstDeleteButton();
                };
            }
        }
    });

    checkForFirstDeleteButton();

    document.getElementById("add-file")!.onclick = () => {
        const template = document.getElementById("editor-template")!;
        const newEditorDom = template.firstElementChild!.cloneNode(true) as HTMLElement;

        // creating the new codemirror editor and append it in the editor div
        editorsjs.push(newEditor(newEditorDom));
        editorsParentdom.append(newEditorDom);
        showDeleteButton(newEditorDom);
    };

    document.querySelector<HTMLFormElement>("form#create")!.onsubmit = () => {
        let j = 0;
        document.querySelectorAll<HTMLInputElement>(".form-filecontent").forEach((el) => {
            if (j < editorsjs.length) {
                el.value = encodeURIComponent(editorsjs[j++].state.doc.toString());
            }
        });

        const fileInput = document.getElementById("file-upload") as HTMLInputElement;
        if (fileInput) {
            fileInput.remove();
        }

        const form = document.querySelector<HTMLFormElement>("form#create")!;

        uploadedFileUUIDs.forEach((fileData) => {
            const uuidInput = document.createElement('input');
            uuidInput.type = 'hidden';
            uuidInput.name = 'uploadedfile_uuid';
            uuidInput.value = fileData.uuid;
            form.appendChild(uuidInput);

            const filenameInput = document.createElement('input');
            filenameInput.type = 'hidden';
            filenameInput.name = 'uploadedfile_filename';
            filenameInput.value = fileData.filename;
            form.appendChild(filenameInput);
        });

        const binaryFiles = document.querySelectorAll('[data-binary-original-name]');
        binaryFiles.forEach((fileDiv) => {
            const originalName = fileDiv.getAttribute('data-binary-original-name');
            const fileNameInput = fileDiv.querySelector('.form-filename') as HTMLInputElement;

            if (fileNameInput) {
                fileNameInput.removeAttribute('name');
            }

            const oldNameInput = document.createElement('input');
            oldNameInput.type = 'hidden';
            oldNameInput.name = 'binary_old_name';
            oldNameInput.value = originalName || '';
            form.appendChild(oldNameInput);

            const newNameInput = document.createElement('input');
            newNameInput.type = 'hidden';
            newNameInput.name = 'binary_new_name';
            newNameInput.value = fileNameInput?.value || '';
            form.appendChild(newNameInput);
        });

        window.onbeforeunload = null;
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

    function checkForFirstDeleteButton() {
        // Count total files (both text and binary)
        const totalFiles = editorsParentdom.querySelectorAll('.editor').length;

        // Hide/show all delete buttons based on total file count
        const deleteButtons = editorsParentdom.querySelectorAll<HTMLButtonElement>("button.delete-file");
        deleteButtons.forEach(deleteBtn => {
            if (totalFiles <= 1) {
                deleteBtn.classList.add("hidden");
                deleteBtn.previousElementSibling?.classList.remove("rounded-l-md");
                deleteBtn.previousElementSibling?.classList.add("rounded-md");
            } else {
                deleteBtn.classList.remove("hidden");
                deleteBtn.previousElementSibling?.classList.add("rounded-l-md");
                deleteBtn.previousElementSibling?.classList.remove("rounded-md");
            }
        });
    }

    function showDeleteButton(editorDom: HTMLElement) {
        let deleteBtn = editorDom.querySelector<HTMLButtonElement>("button.delete-file")!;
        deleteBtn.classList.remove("hidden");
        deleteBtn.previousElementSibling.classList.add("rounded-l-md");
        deleteBtn.previousElementSibling.classList.remove("rounded-md");
        checkForFirstDeleteButton();
    }

    // File upload functionality
    let uploadedFileUUIDs: {uuid: string, filename: string}[] = [];
    const fileUploadInput = document.getElementById("file-upload") as HTMLInputElement;
    const uploadedFilesContainer = document.getElementById("uploaded-files")!;
    const fileUploadZone = document.getElementById("file-upload-zone")!.querySelector('.border-dashed') as HTMLElement;

    // Handle file selection
    const handleFiles = (files: FileList) => {
        Array.from(files).forEach(file => {
            if (!uploadedFileUUIDs.find(f => f.filename === file.name)) {
                uploadFile(file);
            }
        });
    };

    // Upload file to server
    const uploadFile = async (file: File) => {
        const formData = new FormData();
        formData.append('file', file);

        // @ts-ignore
        const baseUrl = window.opengist_base_url || '';
        const csrf = document.querySelector<HTMLInputElement>('form#create input[name="_csrf"]')?.value;

        try {
            const response = await fetch(`${baseUrl}/upload`, {
                method: 'POST',
                credentials: 'same-origin',
                body: formData,
                headers: {
                    'X-CSRF-Token': csrf || ''
                }
            });

            if (response.ok) {
                const result = await response.json();
                uploadedFileUUIDs.push({uuid: result.uuid, filename: result.filename});
                addFileToUI(result.filename, result.uuid, file.size);
            } else {
                console.error('Upload failed:', response.statusText);
            }
        } catch (error) {
            console.error('Upload error:', error);
        }
    };

    // Add file to UI
    const addFileToUI = (filename: string, uuid: string, fileSize: number) => {
        const fileElement = document.createElement('div');
        fileElement.className = 'flex items-stretch bg-gray-50 dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-md overflow-hidden';
        fileElement.dataset.uuid = uuid;

        fileElement.innerHTML = `
            <div class="flex items-center space-x-3 px-3 py-1 flex-1">
                <svg class="h-5 w-5 text-gray-400 dark:text-gray-500" fill="currentColor" viewBox="0 0 20 20">
                    <path d="M4 4a2 2 0 012-2h4.586A2 2 0 0112 2.586L15.414 6A2 2 0 0116 7.414V16a2 2 0 01-2 2H6a2 2 0 01-2-2V4zm2 6a1 1 0 011-1h6a1 1 0 110 2H7a1 1 0 01-1-1zm1 3a1 1 0 100 2h6a1 1 0 100-2H7z"></path>
                </svg>
                <div>
                    <p class="text-sm font-medium text-slate-700 dark:text-slate-300">${filename}</p>
                    <p class="text-xs text-gray-500 dark:text-gray-400">${formatFileSize(fileSize)}</p>
                </div>
            </div>
            <button type="button" class="remove-file flex items-center justify-center px-4 border-l-1 dark:border-l-1 text-rose-600 dark:text-rose-400 border-gray-200 dark:border-gray-600 bg-gray-50 dark:bg-gray-800 hover:bg-rose-500 hover:text-white dark:hover:bg-rose-600 hover:border-rose-600 dark:hover:border-rose-700 dark:hover:text-white focus:outline-none">
                <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                </svg>
            </button>
        `;

        // Remove file handler
        fileElement.querySelector('.remove-file')!.addEventListener('click', async () => {
            // Remove from server
            try {
                // @ts-ignore
                const baseUrl = window.opengist_base_url || '';
                const csrf = document.querySelector<HTMLInputElement>('form#create input[name="_csrf"]')?.value;

                await fetch(`${baseUrl}/upload/${uuid}`, {
                    method: 'DELETE',
                    credentials: 'same-origin',
                    headers: {
                        'X-CSRF-Token': csrf || ''
                    }
                });
            } catch (error) {
                console.error('Error deleting file:', error);
            }

            // Remove from UI and local array
            uploadedFileUUIDs = uploadedFileUUIDs.filter(f => f.uuid !== uuid);
            fileElement.remove();
        });

        uploadedFilesContainer.appendChild(fileElement);
    };

    // Format file size
    const formatFileSize = (bytes: number): string => {
        if (bytes === 0) return '0 B';
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
    };

    // File input change handler
    fileUploadInput.addEventListener('change', (e) => {
        const files = (e.target as HTMLInputElement).files;
        if (files) {
            handleFiles(files);
            // Clear the input value immediately so it doesn't get submitted with the form
            (e.target as HTMLInputElement).value = '';
        }
    });

    // Drag and drop handlers
    fileUploadZone.addEventListener('dragover', (e) => {
        e.preventDefault();
        fileUploadZone.classList.add('border-primary-400', 'dark:border-primary-500');
    });

    fileUploadZone.addEventListener('dragleave', (e) => {
        e.preventDefault();
        fileUploadZone.classList.remove('border-primary-400', 'dark:border-primary-500');
    });

    fileUploadZone.addEventListener('drop', (e) => {
        e.preventDefault();
        fileUploadZone.classList.remove('border-primary-400', 'dark:border-primary-500');
        
        const files = e.dataTransfer?.files;
        if (files) {
            handleFiles(files);
        }
    });

});
