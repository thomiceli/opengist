import PDFObject from 'pdfobject';

// Embed every PDF on the page. PDFObject drops an <iframe> pointing at the raw
// file into the container and the browser's native viewer loads it; we keep the
// server-rendered spinner up until that iframe fires `load`.
//
// PDFObject empties its target node before embedding, so we embed into a child
// holder rather than the `.pdf` element itself — that preserves the spinner
// sibling. The data-pdf-embedded marker keeps this idempotent, so it is safe to
// call again after each hx-boost swap without embedding a second iframe.
export function initPdf() {
  document.querySelectorAll<HTMLElement>('.pdf[data-src]').forEach((el) => {
    if (el.dataset.pdfEmbedded) return;
    el.dataset.pdfEmbedded = 'true';

    const holder = document.createElement('div');
    holder.className = 'pdf-embed';
    el.appendChild(holder);

    const removeSpinner = () => el.querySelector('.pdf-loading')?.remove();
    const embedded = PDFObject.embed(el.dataset.src || '', holder);

    if (embedded && (embedded as HTMLElement).tagName === 'IFRAME') {
      (embedded as HTMLIFrameElement).addEventListener('load', removeSpinner);
    } else {
      // Unsupported browser / fallback markup: nothing will fire `load`.
      removeSpinner();
    }
  });
}
