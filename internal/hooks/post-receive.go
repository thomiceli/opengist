package hooks

import (
	"bufio"
	"fmt"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/git"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
)

func PostReceive(in io.Reader, out, er io.Writer) error {
	opts := pushOptions()
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

	gist, err := db.GetGistByID(os.Getenv("OPENGIST_REPOSITORY_ID"))
	if err != nil {
		_, _ = fmt.Fprintln(er, "Failed to get gist")
		return fmt.Errorf("failed to get gist: %w", err)
	}

	if slices.Contains([]string{"public", "unlisted", "private"}, opts["visibility"]) {
		gist.Private, _ = db.ParseVisibility(opts["visibility"])
		_, _ = fmt.Fprintf(out, "\nGist visibility set to %s\n\n", opts["visibility"])
	}

	if hasNoCommits, err := git.HasNoCommits(gist.User.Username, gist.Uuid); err != nil {
		_, _ = fmt.Fprintln(er, "Failed to check if gist has no commits")
		return fmt.Errorf("failed to check if gist has no commits: %w", err)
	} else if hasNoCommits {
		if err = gist.Delete(); err != nil {
			_, _ = fmt.Fprintln(er, "Failed to delete gist")
			return fmt.Errorf("failed to delete gist: %w", err)
		}
	}

	_ = gist.SetLastActiveNow()
	err = gist.UpdatePreviewAndCount(true)
	if err != nil {
		_, _ = fmt.Fprintln(er, "Failed to update gist")
		return fmt.Errorf("failed to update gist: %w", err)
	}

	gist.AddInIndex()

	return nil
}

func verifyHEAD() error {
	return exec.Command("git", "rev-parse", "--verify", "--quiet", "HEAD").Run()
}

func setSymbolicRef(refname string) {
	_ = exec.Command("git", "symbolic-ref", "HEAD", refname).Run()
}
