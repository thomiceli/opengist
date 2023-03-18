package git

import (
	"bufio"
	"bytes"
	"io"
	"regexp"
)

type File struct {
	Filename    string
	OldFilename string
	Content     string
	Truncated   bool
	IsCreated   bool
	IsDeleted   bool
}

type Commit struct {
	Hash      string
	Author    string
	Timestamp string
	Changed   string
	Files     []File
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
	truncated := len(buf) >= int(maxBytes)
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

func parseLog(out io.Reader) []*Commit {
	scanner := bufio.NewScanner(out)

	var commits []*Commit
	var currentCommit *Commit
	var currentFile *File
	var isContent bool
	var bytesRead = 0

	for scanner.Scan() {
		// new commit found
		currentFile = nil
		currentCommit = &Commit{Hash: string(scanner.Bytes()[2:]), Files: []File{}}

		scanner.Scan()
		currentCommit.Author = string(scanner.Bytes()[2:])

		scanner.Scan()
		currentCommit.Timestamp = string(scanner.Bytes()[2:])

		scanner.Scan()
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
				if bytesRead > 2<<18 {
					currentFile.Truncated = true
					currentFile.Content = ""
					isContent = false
				}
			}

			scanner.Scan()
		}

		if currentCommit != nil {
			commits = append(commits, currentCommit)
		}
	}

	return commits
}
