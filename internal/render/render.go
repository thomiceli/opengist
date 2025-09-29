package render

import (
	"path/filepath"
	"sync"

	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/git"
)

type RenderedFile interface {
	InternalType() string
}

type NonHighlightedFile struct {
	*git.File
	Type string `json:"type"`
}

func (r NonHighlightedFile) InternalType() string {
	return "NonHighlightedFile"
}

func RenderFiles(files []*git.File) []RenderedFile {
	const numWorkers = 10
	jobs := make(chan int, numWorkers)
	renderedFiles := make([]RenderedFile, len(files))
	var wg sync.WaitGroup

	worker := func() {
		for idx := range jobs {
			renderedFiles[idx] = processFile(files[idx])
		}
		wg.Done()
	}

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go worker()
	}

	for i := range files {
		jobs <- i
	}
	close(jobs)

	wg.Wait()

	return renderedFiles
}

func processFile(file *git.File) RenderedFile {
	mt := file.MimeType
	if mt.IsCSV() {
		rendered, err := renderCsvFile(file)
		if err != nil {
			rendered, err := highlightFile(file)
			if err != nil {
				log.Error().Err(err).Msg("Error rendering gist preview for " + file.Filename)
			}
			return rendered
		}
		return rendered
	} else if mt.IsText() && filepath.Ext(file.Filename) == ".md" {
		rendered, err := renderMarkdownFile(file)
		if err != nil {
			log.Error().Err(err).Msg("Error rendering markdown file for " + file.Filename)
		}
		return rendered
	} else if mt.IsSVG() {
		rendered := renderSvgFile(file)
		return rendered
	} else if mt.CanBeEmbedded() {
		rendered := NonHighlightedFile{File: file, Type: mt.RenderType()}
		file.Content = ""
		return rendered
	} else if mt.CanBeRendered() {
		rendered, err := highlightFile(file)
		if err != nil {
			log.Error().Err(err).Msg("Error rendering gist preview for " + file.Filename)
		}
		return rendered
	} else {
		rendered := NonHighlightedFile{File: file, Type: mt.RenderType()}
		file.Content = ""
		return rendered
	}
}
