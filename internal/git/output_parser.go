package git

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"regexp"
	"strings"
)

type File struct {
	Filename    string `json:"filename"`
	Size        uint64 `json:"size"`
	HumanSize   string `json:"human_size"`
	OldFilename string `json:"-"`
	Content     string `json:"content"`
	Truncated   bool   `json:"truncated"`
	IsCreated   bool   `json:"-"`
	IsDeleted   bool   `json:"-"`
}

type CsvFile struct {
	File
	Header []string
	Rows   [][]string
}

type Commit struct {
	Hash        string
	AuthorName  string
	AuthorEmail string
	Timestamp   string
	Changed     string
	Files       []File
}

func truncateCommandOutput(out io.Reader, maxBytes int64) (string, bool, error) {
	var buf []byte
	var err error

	if maxBytes < 0 {
		buf, err = io.ReadAll(out)
	} else {
		buf, err = io.ReadAll(io.LimitReader(out, maxBytes))
	}
	if err != nil {
		return "", false, err
	}
	truncated := maxBytes > 0 && len(buf) >= int(maxBytes)
	// Remove the last line if it's truncated
	if truncated {
		// Find the index of the last newline character
		lastNewline := bytes.LastIndexByte(buf, '\n')

		if lastNewline > 0 {
			// Trim the data buffer up to the last newline character
			buf = buf[:lastNewline]
		}
	}

	return string(buf), truncated, nil
}

func parseLog(out io.Reader, maxBytes int) []*Commit {
	scanner := bufio.NewScanner(out)

	var commits []*Commit
	var currentCommit *Commit
	var currentFile *File
	var isContent bool
	var bytesRead = 0
	scanNext := true

	for {
		if scanNext && !scanner.Scan() {
			break
		}
		scanNext = true

		// new commit found
		currentFile = nil
		currentCommit = &Commit{Hash: string(scanner.Bytes()[2:]), Files: []File{}}

		scanner.Scan()
		currentCommit.AuthorName = string(scanner.Bytes()[2:])

		scanner.Scan()
		currentCommit.AuthorEmail = string(scanner.Bytes()[2:])

		scanner.Scan()
		currentCommit.Timestamp = string(scanner.Bytes()[2:])

		scanner.Scan()

		if len(scanner.Bytes()) == 0 {
			commits = append(commits, currentCommit)
			break
		}

		// if there is no shortstat, it means that the commit is empty, we add it and move onto the next one
		if scanner.Bytes()[0] != ' ' {
			commits = append(commits, currentCommit)

			// avoid scanning the next line, as we already did it
			scanNext = false
			continue
		}

		changed := scanner.Bytes()[1:]
		changed = bytes.ReplaceAll(changed, []byte("(+)"), []byte(""))
		changed = bytes.ReplaceAll(changed, []byte("(-)"), []byte(""))
		currentCommit.Changed = string(changed)

		// twice because --shortstat adds a new line
		scanner.Scan()
		scanner.Scan()
		// commit header parsed

		// files changes inside the commit
		for {
			line := scanner.Bytes()

			// end of content of file
			if len(line) == 0 {
				isContent = false
				if currentFile != nil {
					currentCommit.Files = append(currentCommit.Files, *currentFile)
				}
				break
			}

			// new file found
			if bytes.HasPrefix(line, []byte("diff --git")) {
				// current file is finished, we can add it to the commit
				if currentFile != nil {
					currentCommit.Files = append(currentCommit.Files, *currentFile)
				}

				// create a new file
				isContent = false
				bytesRead = 0
				currentFile = &File{}
				filenameRegex := regexp.MustCompile(`^diff --git a/(.+) b/(.+)$`)
				matches := filenameRegex.FindStringSubmatch(string(line))
				if len(matches) == 3 {
					currentFile.Filename = matches[2]
					if matches[1] != matches[2] {
						currentFile.OldFilename = matches[1]
					}
				}
				scanner.Scan()
				continue
			}

			if bytes.HasPrefix(line, []byte("new")) {
				currentFile.IsCreated = true
			}

			if bytes.HasPrefix(line, []byte("deleted")) {
				currentFile.IsDeleted = true
			}

			// file content found
			if line[0] == '@' {
				isContent = true
			}

			if isContent {
				currentFile.Content += string(line) + "\n"

				bytesRead += len(line)
				if bytesRead > maxBytes {
					currentFile.Truncated = true
					currentFile.Content = ""
					isContent = false
				}
			}

			scanner.Scan()
		}

		commits = append(commits, currentCommit)

	}

	return commits
}

func ParseCsv(file *File) (*CsvFile, error) {

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

	return &CsvFile{
		File:   *file,
		Header: header,
		Rows:   records[1:],
	}, nil
}
