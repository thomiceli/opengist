package render

import (
	"encoding/csv"
	"fmt"
	"strings"

	"github.com/thomiceli/opengist/internal/git"
)

type CSVFile struct {
	*git.File
	Type   string     `json:"type"`
	Header []string   `json:"-"`
	Rows   [][]string `json:"-"`
}

func (r CSVFile) getFile() *git.File {
	return r.File
}

func renderCsvFile(file *git.File) (*CSVFile, error) {
	reader := csv.NewReader(strings.NewReader(file.Content))
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	header := records[0]
	numColumns := len(header)

	for i := 1; i < len(records); i++ {
		if len(records[i]) != numColumns {
			return nil, fmt.Errorf("CSV file has invalid row at index %d", i)
		}
	}

	return &CSVFile{
		File:   file,
		Type:   "CSV",
		Header: header,
		Rows:   records[1:],
	}, nil
}
