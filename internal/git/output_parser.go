package git

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
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

// inspired from https://github.com/go-gitea/gitea/blob/main/services/gitdiff/gitdiff.go
func parseLog(out io.Reader, maxFiles int, maxBytes int) ([]*Commit, error) {
	var commits []*Commit
	var currentCommit *Commit
	var currentFile *File
	var headerParsed = false
	var skipped = false
	var line string
	var err error

	input := bufio.NewReaderSize(out, maxBytes)

	// Loop Commits
loopLog:
	for {
		// If a commit was skipped, do not read a new line
		if !skipped {
			line, err = input.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					break loopLog
				}
				return commits, err
			}
		}

		// Remove trailing newline characters
		if len(line) > 0 && (line[len(line)-1] == '\n' || line[len(line)-1] == '\r') {
			line = line[:len(line)-1]
		}

		// Attempt to parse commit header (hash, author, mail, timestamp) or a diff
		switch line[0] {
		// Commit hash
		case 'c':
			if headerParsed {
				commits = append(commits, currentCommit)
			}
			skipped = false
			currentCommit = &Commit{Hash: line[2:], Files: []File{}}
			continue

		// Author name
		case 'a':
			headerParsed = true
			currentCommit.AuthorName = line[2:]
			continue

		// Author email
		case 'm':
			currentCommit.AuthorEmail = line[2:]
			continue

		// Commit timestamp
		case 't':
			currentCommit.Timestamp = line[2:]
			continue

		// Commit shortstat
		case ' ':
			changed := []byte(line)[1:]
			changed = bytes.ReplaceAll(changed, []byte("(+)"), []byte(""))
			changed = bytes.ReplaceAll(changed, []byte("(-)"), []byte(""))
			currentCommit.Changed = string(changed)

			// shortstat is followed by an empty line
			line, err = input.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					break loopLog
				}
				return commits, err
			}
			continue

		// Commit diff
		default:
			// Loop files in diff
		loopCommit:
			for {
				// If we have reached the maximum number of files to show for a single commit, skip to the next commit
				if len(currentCommit.Files) >= maxFiles {
					line, err = skipToNextCommit(input)
					if err != nil {
						if err == io.EOF {
							break loopLog
						}
						return commits, err
					}

					// Skip to the next commit
					headerParsed = false
					skipped = true
					break loopCommit
				}

				// Else create a new file and parse it
				currentFile = &File{}
				parseRename := true

			loopFileDiff:
				for {
					line, err = input.ReadString('\n')
					if err != nil {
						if err != io.EOF {
							return commits, err
						}
						headerParsed = false
						break loopCommit
					}

					// If the line is a newline character, the commit is finished
					if line == "\n" {
						currentCommit.Files = append(currentCommit.Files, *currentFile)
						headerParsed = false
						break loopCommit
					}

					// Attempt to parse the file header
					switch {
					case strings.HasPrefix(line, "diff --git"):
						currentCommit.Files = append(currentCommit.Files, *currentFile)
						headerParsed = false
						break loopFileDiff
					case strings.HasPrefix(line, "old mode"):
					case strings.HasPrefix(line, "new mode"):
					case strings.HasPrefix(line, "index"):
					case strings.HasPrefix(line, "similarity index"):
					case strings.HasPrefix(line, "dissimilarity index"):
						continue
					case strings.HasPrefix(line, "rename from "):
						currentFile.OldFilename = line[12 : len(line)-1]
					case strings.HasPrefix(line, "rename to "):
						currentFile.Filename = line[10 : len(line)-1]
						parseRename = false
					case strings.HasPrefix(line, "copy from "):
						currentFile.OldFilename = line[10 : len(line)-1]
					case strings.HasPrefix(line, "copy to "):
						currentFile.Filename = line[8 : len(line)-1]
						parseRename = false
					case strings.HasPrefix(line, "new file"):
						currentFile.IsCreated = true
					case strings.HasPrefix(line, "deleted file"):
						currentFile.IsDeleted = true
					case strings.HasPrefix(line, "--- "):
						name := line[4 : len(line)-1]
						if parseRename && currentFile.IsDeleted {
							currentFile.Filename = name[2:]
						} else if parseRename && strings.HasPrefix(name, "a/") {
							currentFile.OldFilename = name[2:]
						}
					case strings.HasPrefix(line, "+++ "):
						name := line[4 : len(line)-1]
						if parseRename && strings.HasPrefix(name, "b/") {
							currentFile.Filename = name[2:]
						}

						// Header is finally parsed, now we can parse the file diff content
						lineBytes, isFragment, err := parseDiffContent(currentFile, maxBytes, input)
						if err != nil {
							if err != io.EOF {
								return commits, err
							}

							// EOF reached, commit is finished
							currentCommit.Files = append(currentCommit.Files, *currentFile)
							headerParsed = false
							break loopCommit
						}

						currentCommit.Files = append(currentCommit.Files, *currentFile)

						if string(lineBytes) == "" {
							headerParsed = false
							break loopCommit
						}

						for isFragment {
							_, isFragment, err = input.ReadLine()
							if err != nil {
								return commits, fmt.Errorf("unable to ReadLine: %w", err)
							}
						}

						break loopFileDiff
					}
				}
			}
		}
		commits = append(commits, currentCommit)
	}

	return commits, nil
}

func parseDiffContent(currentFile *File, maxBytes int, input *bufio.Reader) (lineBytes []byte, isFragment bool, err error) {
	sb := &strings.Builder{}
	var currFileLineCount int

	for {
		for isFragment {
			currentFile.Truncated = true

			// Read the next line
			_, isFragment, err = input.ReadLine()
			if err != nil {
				return nil, false, err
			}
		}

		sb.Reset()

		// Read the next line
		lineBytes, isFragment, err = input.ReadLine()
		if err != nil {
			if err == io.EOF {
				return lineBytes, isFragment, err
			}
			return nil, false, err
		}

		// End of file
		if len(lineBytes) == 0 {
			return lineBytes, false, err
		}
		if lineBytes[0] == 'd' {
			return lineBytes, false, err
		}

		if currFileLineCount >= maxBytes {
			currentFile.Truncated = true
			continue
		}

		line := string(lineBytes)
		if isFragment {
			currentFile.Truncated = true
			for isFragment {
				lineBytes, isFragment, err = input.ReadLine()
				if err != nil {
					return lineBytes, isFragment, fmt.Errorf("unable to ReadLine: %w", err)
				}
			}
		}

		if len(line) > maxBytes {
			currentFile.Truncated = true
			line = line[:maxBytes]
		}
		currentFile.Content += line + "\n"
	}
}

func skipToNextCommit(input *bufio.Reader) (line string, err error) {
	// need to skip until the next cmdDiffHead
	var isFragment, wasFragment bool
	var lineBytes []byte
	for {
		lineBytes, isFragment, err = input.ReadLine()
		if err != nil {
			return "", err
		}
		if wasFragment {
			wasFragment = isFragment
			continue
		}
		if bytes.HasPrefix(lineBytes, []byte("c")) {
			break
		}
		wasFragment = isFragment
	}
	line = string(lineBytes)
	if isFragment {
		var tail string
		tail, err = input.ReadString('\n')
		if err != nil {
			return "", err
		}
		line += tail
	}
	return line, err
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
