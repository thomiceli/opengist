package hooks

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/thomiceli/opengist/internal/ipc"
)

// PreReceive is the client side of the pre-receive hook. It runs in the
// short-lived hook subprocess, where Git's push quarantine makes the incoming
// objects visible, so it computes the changed files locally and forwards them to
// the running daemon's internal API, which applies the push policy. It opens no
// database.
func PreReceive(in io.Reader, out, er io.Writer) error {
	var changedFilesPerRef []string

	scanner := bufio.NewScanner(in)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, " ")
		if len(parts) < 3 {
			_, _ = fmt.Fprintln(er, "Invalid input")
			return fmt.Errorf("invalid input")
		}

		oldRev, newRev := parts[0], parts[1]
		if newRev != BaseHash {
			symlinkFiles, err := getSymlinkFiles(newRev)
			if err != nil {
				_, _ = fmt.Fprintln(er, "Failed to inspect pushed files")
				return err
			}
			if len(symlinkFiles) > 0 {
				_, _ = fmt.Fprintln(out, "\nPushing symbolic links is not allowed:")
				for _, filename := range symlinkFiles {
					_, _ = fmt.Fprintf(out, "  %q\n", filename)
				}
				_, _ = fmt.Fprintln(out)
				return fmt.Errorf("push contains symbolic links")
			}
		}

		var rev string
		if oldRev == BaseHash {
			// First commit
			rev = newRev
		} else {
			rev = fmt.Sprintf("%s..%s", oldRev, newRev)
		}

		changedFiles, err := getChangedFiles(rev)
		if err != nil {
			_, _ = fmt.Fprintln(er, "Failed to get changed files")
			return err
		}
		changedFilesPerRef = append(changedFilesPerRef, changedFiles)
	}

	resp, err := ipc.HookPreReceive(&ipc.HookPreReceiveRequest{ChangedFiles: changedFilesPerRef})
	if err != nil {
		_, _ = fmt.Fprintln(er, err.Error())
		return err
	}

	if !resp.Allowed {
		_, _ = fmt.Fprint(out, resp.Message)
		return fmt.Errorf("push rejected")
	}

	return nil
}

// RunPreReceive is the server side of the pre-receive hook. It runs inside the
// daemon and decides whether a push may proceed, given the changed files the
// subprocess computed (one raw `git log` output per ref). Opengist gists are
// flat, so pushing files inside directories is rejected.
func RunPreReceive(changedFilesPerRef []string) (bool, string) {
	var disallowedFiles []string
	var disallowedCommits []string

	for _, changedFiles := range changedFilesPerRef {
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
		var sb strings.Builder
		_, _ = fmt.Fprintln(&sb, "\nPushing files in directories is not allowed:")
		for i := range disallowedFiles {
			_, _ = fmt.Fprintf(&sb, "  %s (%s)\n", disallowedFiles[i], disallowedCommits[i])
		}
		_, _ = fmt.Fprintln(&sb)
		return false, sb.String()
	}

	return true, ""
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

func getSymlinkFiles(rev string) ([]string, error) {
	cmd := exec.Command("git", "ls-tree", "-rz", "-r", "--full-tree", "--end-of-options", rev)

	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	return parseSymlinkFiles(out), nil
}

func parseSymlinkFiles(output []byte) []string {
	var symlinkFiles []string
	for _, record := range bytes.Split(output, []byte{0}) {
		tab := bytes.IndexByte(record, '\t')
		if tab < 0 {
			continue
		}

		metadata := bytes.Fields(record[:tab])
		if len(metadata) > 0 && bytes.Equal(metadata[0], []byte("120000")) {
			symlinkFiles = append(symlinkFiles, string(record[tab+1:]))
		}
	}

	return symlinkFiles
}
