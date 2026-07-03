import hljs from 'highlight.js';
import latex from './latex';
import { marked } from 'marked';
import DOMPurify from 'dompurify';

// Notebook content is attacker-controlled: any user can store a `.ipynb` gist
// whose JSON ends up injected into the page. Every fragment that reaches an
// `innerHTML` sink must therefore be sanitized to prevent stored XSS (see the
// security advisory describing the original markdown-cell / output sinks).
const sanitize = (html: string): string => DOMPurify.sanitize(html);

// nbformat allows `source`/`text` fields to be either a string or an array of strings.
const joinSource = (source: string | string[]): string => Array.isArray(source) ? source.join('') : source || '';

class IPynb {
  private element: HTMLElement;
  private cells: HTMLElement[] = [];
  private language: string = 'python';
  private notebook: any;

  constructor(element: HTMLElement) {
    this.element = element;
    // textContent yields the raw notebook JSON the server escaped into the
    // <pre>; it is parsed as data, never injected as markup.
    let notebookContent = element.textContent || '';

    try {
      this.notebook = JSON.parse(notebookContent);
    } catch (e) {
      console.error('Failed to parse Jupyter notebook content:', e);
      return;
    }

    if (!this.notebook) {
      console.error('Failed to parse Jupyter notebook content:', notebookContent);
      return;
    }

    this.language = this.notebook.metadata.kernelspec?.language || 'python';
    this.cells = this.createCells();
  }

  mount() {
    const parent = this.element.parentElement as HTMLElement;
    parent.removeChild(this.element);
    parent.innerHTML = this.cells
      .filter((cell: HTMLElement) => !!cell?.outerHTML)
      .map((cell: HTMLElement) => cell.outerHTML)
      .join('');
  }

  private getOutputs(cell: any): HTMLElement[] {
    return (cell.outputs || []).map((output: any) => {
      const outputElement = document.createElement('div');
      outputElement.classList.add('jupyter-output');

      if (output.output_type === 'stream') {
        const textElement = document.createElement('pre');
        textElement.classList.add('stream-output');
        textElement.textContent = joinSource(output.text);
        outputElement.appendChild(textElement);
      } else if (output.output_type === 'display_data' || output.output_type === 'execute_result') {
        if (output.data['text/plain']) {
          outputElement.innerHTML += sanitize(`\n<pre>${output.data['text/plain']}</pre>`);
        }
        if (output.data['text/html']) {
          outputElement.innerHTML += '\n' + sanitize(output.data['text/html']);
        }

        const images = Object.keys(output.data).filter(key => key.startsWith('image/'));
        if (images.length > 0) {
          const imgEl = document.createElement('img');
          const imgType = images[0]; // Use the first image type found
          imgEl.src = `data:${imgType};base64,${output.data[imgType]}`;
          outputElement.innerHTML += sanitize(imgEl.outerHTML);
        }
      } else if (output.output_type === 'error') {
        outputElement.classList.add('error');
        outputElement.textContent = `Error: ${output.ename}: ${output.evalue}`;
      }

      return outputElement;
    });
  }

  private createCellElement(cell: any): HTMLElement {
    const cellElement = document.createElement('div');
    const source = joinSource(cell.source);
    cellElement.classList.add('jupyter-cell');

    switch (cell.cell_type) {
      case 'markdown':
        cellElement.classList.add('markdown-cell');
        cellElement.innerHTML = sanitize(
          `<div class="markdown-body">${marked.parse(latex.render(source)) as string}</div>`
        );
        break;
      case 'code': {
        cellElement.classList.add('code-cell');
        // Build the code block via DOM APIs so the source is treated as text,
        // not markup, before highlight.js processes it.
        const pre = document.createElement('pre');
        pre.classList.add('hljs');
        const code = document.createElement('code');
        code.classList.add(`language-${this.language}`);
        code.textContent = source;
        pre.appendChild(code);
        cellElement.appendChild(pre);
        hljs.highlightElement(code);
        break;
      }
      default:
        break;
    }

    return cellElement;
  }


  private createCells(): HTMLElement[] {
    return (this.notebook.cells || []).map((cell: any) => {
      const container = document.createElement('div');
      const cellElement = this.createCellElement(cell);
      const outputs = this.getOutputs(cell);

      container.classList.add('jupyter-cell-container');
      container.appendChild(cellElement);
      outputs.forEach((output: HTMLElement) => container.appendChild(output));
      return container;
    });
  }
}

// Process Jupyter notebooks
document.querySelectorAll<HTMLElement>('.jupyter.notebook pre').forEach((el) => {
  new IPynb(el).mount();
});
