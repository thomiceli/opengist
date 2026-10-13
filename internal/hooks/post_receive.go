package hooks

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"

	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/git"
	"github.com/thomiceli/opengist/internal/ipc"
	validatorpkg "github.com/thomiceli/opengist/internal/validator"
)

// PostReceive is the client side of the post-receive hook. It runs in the
// short-lived hook subprocess: it gathers the ref updates from stdin and the
// push options from the environment, forwards them to the running daemon's
// internal API (which holds the warm database connection and the index), and
// prints back whatever message the daemon produced. It opens no database.
func PostReceive(in io.Reader, out, er io.Writer) error {
	gistID := os.Getenv("OPENGIST_REPOSITORY_ID")
	if gistID == "" {
		// No gist id is set on the SSH push path, which performs the gist update
		// in-process instead. Nothing for this hook to forward.
		return nil
	}

	var refs []ipc.HookRefUpdate

	scanner := bufio.NewScanner(in)
	for scanner.Scan() {
		parts := strings.Fields(scanner.Text())
		if len(parts) != 3 {
			_, _ = fmt.Fprintln(er, "Invalid input")
			return fmt.Errorf("invalid input")
		}
		refs = append(refs, ipc.HookRefUpdate{OldRev: parts[0], NewRev: parts[1], RefName: parts[2]})
	}

	resp, err := ipc.HookPostReceive(&ipc.HookPostReceiveRequest{
		GistID:      gistID,
		GistURL:     os.Getenv("OPENGIST_REPOSITORY_URL_INTERNAL"),
		References:  refs,
		PushOptions: pushOptions(),
	})
	if err != nil {
		_, _ = fmt.Fprintln(er, err.Error())
		return err
	}

	if resp.Output != "" {
		_, _ = fmt.Fprint(out, "\n"+resp.Output)
	}

	return nil
}

// RunPostReceive is the server side of the post-receive hook. It runs inside the
// daemon (warm database connection and search index) on behalf of the hook
// subprocess, and returns the message to show the user pushing.
func RunPostReceive(gist *db.Gist, repoDir, gistUrl string, refs []ipc.HookRefUpdate, opts map[string]string) (string, error) {
	var outputSb strings.Builder
	newGist := false
	validator := validatorpkg.NewValidator()

	for _, ref := range refs {
		if err := verifyHEAD(repoDir); err != nil {
			setSymbolicRef(repoDir, ref.RefName)
		}

		if ref.OldRev == BaseHash {
			newGist = true
		}
	}

	if gistUrl == "" {
		gistUrl = strings.TrimSuffix(config.C.ExternalUrl, "/") + "/" + gist.User.Username + "/" + gist.Identifier()
	}

	if slices.Contains([]string{"public", "unlisted", "private"}, opts["visibility"]) {
		gist.Private = db.ParseVisibility(opts["visibility"])
		fmt.Fprintf(&outputSb, "Gist visibility set to %s\n\n", opts["visibility"])
	}

	if opts["url"] != "" && validator.Var(opts["url"], "max=32,alphanumdashorempty") == nil {
		gist.URL = opts["url"]
		lastIndex := strings.LastIndex(gistUrl, "/")
		gistUrl = gistUrl[:lastIndex+1] + gist.URL
		if !newGist {
			fmt.Fprintf(&outputSb, "Gist URL set to %s. Set the Git remote URL via:\n", gistUrl)
			fmt.Fprintf(&outputSb, "git remote set-url origin %s\n\n", gistUrl)
		}
	}

	if opts["title"] != "" && validator.Var(opts["title"], "max=250") == nil {
		gist.Title = opts["title"]
		fmt.Fprintf(&outputSb, "Gist title set to \"%s\"\n\n", opts["title"])
	}

	if opts["description"] != "" && validator.Var(opts["description"], "max=1000") == nil {
		gist.Description = opts["description"]
		fmt.Fprintf(&outputSb, "Gist description set to \"%s\"\n\n", opts["description"])
	}

	if opts["topics"] != "" && validator.Var(opts["topics"], "gisttopics") == nil {
		topicNames := strings.Fields(opts["topics"])
		if len(topicNames) > 0 {
			gist.Topics = make([]db.GistTopic, 0, len(topicNames))
			for _, name := range topicNames {
				gist.Topics = append(gist.Topics, db.GistTopic{Topic: name})
			}
			fmt.Fprintf(&outputSb, "Gist topics set to \"%s\"\n\n", opts["topics"])
		}
	}

	if newGist && opts["expire"] != "" {
		value := opts["expire"]
		expire := db.ExpirationType(value)
		switch {
		case expire == db.ExpiryNever:
			// no expiration
		case expire.Duration() > 0:
			gist.ExpiresAt = expire.ExpiresAtTimestamp()
			fmt.Fprintf(&outputSb, "Gist expiration set to \"%s\"\n\n", value)
		default:
			if t, err := validatorpkg.ParseDateTime(value); err == nil && t.After(time.Now()) {
				gist.ExpiresAt = t.Unix()
				fmt.Fprintf(&outputSb, "Gist expiration set to \"%s\"\n\n", value)
			} else {
				fmt.Fprintf(&outputSb, "Invalid gist expiration \"%s\", ignored\n\n", value)
			}
		}
	}

	if hasNoCommits, err := git.HasNoCommits(gist.User.Username, gist.Uuid); err != nil {
		return "", fmt.Errorf("failed to check if gist has no commits: %w", err)
	} else if hasNoCommits {
		if err = gist.Delete(); err != nil {
			return "", fmt.Errorf("failed to delete gist: %w", err)
		}
	}

	_ = gist.SetLastActiveNow()
	if err := gist.UpdatePreviewAndCount(true); err != nil {
		return "", fmt.Errorf("failed to update gist: %w", err)
	}

	gist.AddInIndex()

	if newGist {
		fmt.Fprintf(&outputSb, "Your new gist has been created here: %s\n", gistUrl)
		outputSb.WriteString("If you want to keep working with your gist, you could set the Git remote URL via:\n")
		fmt.Fprintf(&outputSb, "git remote set-url origin %s\n\n", gistUrl)
	}

	return outputSb.String(), nil
}

func verifyHEAD(dir string) error {
	cmd := exec.Command("git", "rev-parse", "--verify", "--quiet", "HEAD")
	cmd.Dir = dir
	return cmd.Run()
}

func setSymbolicRef(dir, refname string) {
	cmd := exec.Command("git", "symbolic-ref", "HEAD", refname)
	cmd.Dir = dir
	_ = cmd.Run()
}
