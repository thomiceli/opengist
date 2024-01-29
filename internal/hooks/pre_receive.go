package hooks

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

func PreReceive(in io.Reader, out, er io.Writer) error {
	var err error
	var disallowedFiles []string
	var disallowedCommits []string

	scanner := bufio.NewScanner(in)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, " ")
		if len(parts) < 3 {
			_, _ = fmt.Fprintln(er, "Invalid input")
			return fmt.Errorf("invalid input")
		}

		oldRev, newRev := parts[0], parts[1]

		var changedFiles string
		if oldRev == BaseHash {
			// First commit
			if changedFiles, err = getChangedFiles(newRev); err != nil {
				_, _ = fmt.Fprintln(er, "Failed to get changed files")
				return err
			}
		} else {
			if changedFiles, err = getChangedFiles(fmt.Sprintf("%s..%s", oldRev, newRev)); err != nil {
				_, _ = fmt.Fprintln(er, "Failed to get changed files")
				return err
			}
		}

		var currentCommit string
		for _, file := range strings.Fields(changedFiles) {
			if strings.HasPrefix(file, "/") {
				currentCommit = file[1:]
			}

			if strings.Contains(file[1:], "/") {
				disallowedFiles = append(disallowedFiles, file)
				disallowedCommits = append(disallowedCommits, currentCommit[0:7])
			}
		}
	}

	if len(disallowedFiles) > 0 {
		_, _ = fmt.Fprintln(out, "\nPushing files in directories is not allowed:")
		for i := range disallowedFiles {
			_, _ = fmt.Fprintf(out, "  %s (%s)\n", disallowedFiles[i], disallowedCommits[i])
		}
		_, _ = fmt.Fprintln(out)
		return fmt.Errorf("pushing files in directories is not allowed: %s", disallowedFiles)
	}

	return nil
}

func getChangedFiles(rev string) (string, error) {
	cmd := exec.Command("git", "log", "--name-only", "--format=/%H", "--diff-filter=AM", rev)

	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", err
	}

	return out.String(), nil
}
