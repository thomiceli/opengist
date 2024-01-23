package hooks

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

func PostReceive(in io.Reader, out, er io.Writer) error {
	scanner := bufio.NewScanner(in)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) != 3 {
			_, _ = fmt.Fprintln(er, "Invalid input")
			return fmt.Errorf("invalid input")
		}
		oldrev, _, refname := parts[0], parts[1], parts[2]

		if err := verifyHEAD(); err != nil {
			setSymbolicRef(refname)
		}

		if oldrev == BaseHash {
			_, _ = fmt.Fprintf(out, "\nYour new repository has been created here: %s\n\n", os.Getenv("OPENGIST_REPOSITORY_URL_INTERNAL"))
			_, _ = fmt.Fprintln(out, "If you want to keep working with your gist, you could set the remote URL via:")
			_, _ = fmt.Fprintf(out, "git remote set-url origin %s\n\n", os.Getenv("OPENGIST_REPOSITORY_URL_INTERNAL"))
		}
	}

	return nil
}

func verifyHEAD() error {
	return exec.Command("git", "rev-parse", "--verify", "--quiet", "HEAD").Run()
}

func setSymbolicRef(refname string) {
	_ = exec.Command("git", "symbolic-ref", "HEAD", refname).Run()
}
