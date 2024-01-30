package hooks

import (
	"bufio"
	"fmt"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/git"
	"github.com/thomiceli/opengist/internal/utils"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
)

func PostReceive(in io.Reader, out, er io.Writer) error {
	var outputSb strings.Builder
	newGist := false
	opts := pushOptions()
	gistUrl := os.Getenv("OPENGIST_REPOSITORY_URL_INTERNAL")
	validator := utils.NewValidator()

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
			newGist = true
		}
	}

	gist, err := db.GetGistByID(os.Getenv("OPENGIST_REPOSITORY_ID"))
	if err != nil {
		_, _ = fmt.Fprintln(er, "Failed to get gist")
		return fmt.Errorf("failed to get gist: %w", err)
	}

	if slices.Contains([]string{"public", "unlisted", "private"}, opts["visibility"]) {
		gist.Private, _ = db.ParseVisibility(opts["visibility"])
		outputSb.WriteString(fmt.Sprintf("Gist visibility set to %s\n\n", opts["visibility"]))
	}

	if opts["url"] != "" && validator.Var(opts["url"], "max=32,alphanumdashorempty") == nil {
		gist.URL = opts["url"]
		lastIndex := strings.LastIndex(gistUrl, "/")
		gistUrl = gistUrl[:lastIndex+1] + gist.URL
		if !newGist {
			outputSb.WriteString(fmt.Sprintf("Gist URL set to %s. Set the Git remote URL via:\n", gistUrl))
			outputSb.WriteString(fmt.Sprintf("git remote set-url origin %s\n\n", gistUrl))
		}
	}

	if opts["title"] != "" && validator.Var(opts["title"], "max=250") == nil {
		gist.Title = opts["title"]
		outputSb.WriteString(fmt.Sprintf("Gist title set to \"%s\"\n\n", opts["title"]))
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

	if newGist {
		outputSb.WriteString(fmt.Sprintf("Your new gist has been created here: %s\n", gistUrl))
		outputSb.WriteString("If you want to keep working with your gist, you could set the Git remote URL via:\n")
		outputSb.WriteString(fmt.Sprintf("git remote set-url origin %s\n\n", gistUrl))
	}

	outputStr := outputSb.String()
	if outputStr != "" {
		_, _ = fmt.Fprint(out, "\n"+outputStr)
	}

	return nil
}

func verifyHEAD() error {
	return exec.Command("git", "rev-parse", "--verify", "--quiet", "HEAD").Run()
}

func setSymbolicRef(refname string) {
	_ = exec.Command("git", "symbolic-ref", "HEAD", refname).Run()
}
