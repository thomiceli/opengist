import hljs from 'highlight.js';
import latex from './latex';
import showdown from 'showdown';

class IPynb {
  private element: HTMLElement;
  private cells: HTMLElement[] = [];
  private language: string = 'python';
  private notebook: any;

  constructor(element: HTMLElement) {
    this.element = element;
    let notebookContent = element.innerText;

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
        outputElement.textContent = output.text.join('\n');
      } else if (output.output_type === 'display_data' || output.output_type === 'execute_result') {
        if (output.data['text/plain']) {
          outputElement.innerHTML += `\n<pre>${output.data['text/plain']}</pre>`;
        }
        if (output.data['text/html']) {
          outputElement.innerHTML += '\n' + output.data['text/html'];
        }

        const images = Object.keys(output.data).filter(key => key.startsWith('image/'));
        if (images.length > 0) {
          const imgEl = document.createElement('img');
          const imgType = images[0]; // Use the first image type found
          imgEl.src = `data:${imgType};base64,${output.data[imgType]}`;
          outputElement.innerHTML += imgEl.outerHTML;
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
    const source = cell.source.join('');
    cellElement.classList.add('jupyter-cell');

    switch (cell.cell_type) {
      case 'markdown':
        const converter = new showdown.Converter();
        cellElement.classList.add('markdown-cell');
        cellElement.innerHTML = `<div class="markdown-body">${converter.makeHtml(latex.render(source))}</div>`;
        break;
      case 'code':
        cellElement.classList.add('code-cell');
        cellElement.innerHTML = `<pre class="hljs"><code class="language-${this.language}">${source}</code></pre>`;
        hljs.highlightElement(cellElement.querySelector('code') as HTMLElement);
        break;
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
