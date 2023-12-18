package git

import (
	"bytes"
	"context"
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/config"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var (
	ReposDirectory = "repos"
)

const truncateLimit = 2 << 18

func RepositoryPath(user string, gist string) string {
	return filepath.Join(config.GetHomeDir(), ReposDirectory, strings.ToLower(user), gist)
}

func RepositoryUrl(ctx echo.Context, user string, gist string) string {
	httpProtocol := "http"
	if ctx.Request().TLS != nil || ctx.Request().Header.Get("X-Forwarded-Proto") == "https" {
		httpProtocol = "https"
	}

	var baseHttpUrl string
	// if a custom external url is set, use it
	if config.C.ExternalUrl != "" {
		baseHttpUrl = config.C.ExternalUrl
	} else {
		baseHttpUrl = httpProtocol + "://" + ctx.Request().Host
	}

	return fmt.Sprintf("%s/%s/%s", baseHttpUrl, user, gist)
}

func TmpRepositoryPath(gistId string) string {
	dirname := TmpRepositoriesPath()
	return filepath.Join(dirname, gistId)
}

func TmpRepositoriesPath() string {
	return filepath.Join(config.GetHomeDir(), "tmp", "repos")
}

func InitRepository(user string, gist string) error {
	repositoryPath := RepositoryPath(user, gist)

	cmd := exec.Command(
		"git",
		"init",
		"--bare",
		repositoryPath,
	)

	if err := cmd.Run(); err != nil {
		return err
	}

	return createDotGitFiles(repositoryPath)
}

func InitRepositoryViaInit(user string, gist string, ctx echo.Context) error {
	repositoryPath := RepositoryPath(user, gist)

	if err := InitRepository(user, gist); err != nil {
		return err
	}
	repositoryUrl := RepositoryUrl(ctx, user, gist)
	return createDotGitHookFile(repositoryPath, "post-receive", fmt.Sprintf(postReceive, repositoryUrl, repositoryUrl))
}

func CountCommits(user string, gist string) (string, error) {
	repositoryPath := RepositoryPath(user, gist)

	cmd := exec.Command(
		"git",
		"rev-list",
		"--all",
		"--count",
	)
	cmd.Dir = repositoryPath

	stdout, err := cmd.Output()
	return strings.TrimSuffix(string(stdout), "\n"), err
}

func GetFilesOfRepository(user string, gist string, revision string) ([]string, error) {
	repositoryPath := RepositoryPath(user, gist)

	cmd := exec.Command(
		"git",
		"ls-tree",
		"--name-only",
		"--",
		revision,
	)
	cmd.Dir = repositoryPath

	stdout, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	slice := strings.Split(string(stdout), "\n")
	return slice[:len(slice)-1], nil
}

func GetFileContent(user string, gist string, revision string, filename string, truncate bool) (string, bool, error) {
	repositoryPath := RepositoryPath(user, gist)

	var maxBytes int64 = -1
	if truncate {
		maxBytes = truncateLimit
	}

	// Set up a context with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		"git",
		"--no-pager",
		"show",
		revision+":"+filename,
	)
	cmd.Dir = repositoryPath

	output, err := cmd.Output()
	if err != nil {
		return "", false, err
	}

	content, truncated, err := truncateCommandOutput(bytes.NewReader(output), maxBytes)
	if err != nil {
		return "", false, err
	}

	return content, truncated, nil
}

func GetFileSize(user string, gist string, revision string, filename string) (int64, error) {
	repositoryPath := RepositoryPath(user, gist)

	cmd := exec.Command(
		"git",
		"cat-file",
		"-s",
		revision+":"+filename,
	)
	cmd.Dir = repositoryPath

	stdout, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	return strconv.ParseInt(strings.TrimSuffix(string(stdout), "\n"), 10, 64)
}

func GetLog(user string, gist string, skip int) ([]*Commit, error) {
	repositoryPath := RepositoryPath(user, gist)

	cmd := exec.Command(
		"git",
		"--no-pager",
		"log",
		"-n",
		"11",
		"--no-color",
		"-p",
		"--skip",
		strconv.Itoa(skip),
		"--format=format:c %H%na %aN%nm %ae%nt %at",
		"--shortstat",
		"HEAD",
	)
	cmd.Dir = repositoryPath
	stdout, _ := cmd.StdoutPipe()
	err := cmd.Start()
	if err != nil {
		return nil, err
	}
	defer func(cmd *exec.Cmd) {
		waitErr := cmd.Wait()
		if waitErr != nil {
			err = waitErr
		}
	}(cmd)

	return parseLog(stdout, truncateLimit), err
}

func CloneTmp(user string, gist string, gistTmpId string, email string) error {
	repositoryPath := RepositoryPath(user, gist)

	tmpPath := TmpRepositoriesPath()

	tmpRepositoryPath := path.Join(tmpPath, gistTmpId)

	err := os.RemoveAll(tmpRepositoryPath)
	if err != nil {
		return err
	}

	cmd := exec.Command("git", "clone", repositoryPath, gistTmpId)
	cmd.Dir = tmpPath
	if err = cmd.Run(); err != nil {
		return err
	}

	// remove every file (and not the .git directory!)
	if err = removeFilesExceptGit(tmpRepositoryPath); err != nil {
		return err
	}

	cmd = exec.Command("git", "config", "--local", "user.name", user)
	cmd.Dir = tmpRepositoryPath
	if err = cmd.Run(); err != nil {
		return err
	}

	cmd = exec.Command("git", "config", "--local", "user.email", email)
	cmd.Dir = tmpRepositoryPath
	return cmd.Run()
}

func ForkClone(userSrc string, gistSrc string, userDst string, gistDst string) error {
	repositoryPathSrc := RepositoryPath(userSrc, gistSrc)
	repositoryPathDst := RepositoryPath(userDst, gistDst)

	cmd := exec.Command("git", "clone", "--bare", repositoryPathSrc, repositoryPathDst)
	if err := cmd.Run(); err != nil {
		return err
	}

	return createDotGitFiles(repositoryPathDst)
}

func SetFileContent(gistTmpId string, filename string, content string) error {
	repositoryPath := TmpRepositoryPath(gistTmpId)

	return os.WriteFile(filepath.Join(repositoryPath, filename), []byte(content), 0644)
}

func AddAll(gistTmpId string) error {
	tmpPath := TmpRepositoryPath(gistTmpId)

	// in case of a change where only a file name has its case changed
	cmd := exec.Command("git", "rm", "-r", "--cached", "--ignore-unmatch", ".")
	cmd.Dir = tmpPath
	err := cmd.Run()
	if err != nil {
		return err
	}

	cmd = exec.Command("git", "add", "-A")
	cmd.Dir = tmpPath

	return cmd.Run()
}

func CommitRepository(gistTmpId string, authorName string, authorEmail string) error {
	cmd := exec.Command("git",
		"commit",
		"--allow-empty",
		"-m",
		"Opengist commit",
		"--author",
		fmt.Sprintf("%s <%s>", authorName, authorEmail),
	)
	tmpPath := TmpRepositoryPath(gistTmpId)
	cmd.Dir = tmpPath

	return cmd.Run()
}

func Push(gistTmpId string) error {
	tmpRepositoryPath := TmpRepositoryPath(gistTmpId)
	cmd := exec.Command(
		"git",
		"push",
	)
	cmd.Dir = tmpRepositoryPath

	err := cmd.Run()
	if err != nil {
		return err
	}

	return os.RemoveAll(tmpRepositoryPath)
}

func DeleteRepository(user string, gist string) error {
	return os.RemoveAll(RepositoryPath(user, gist))
}

func UpdateServerInfo(user string, gist string) error {
	repositoryPath := RepositoryPath(user, gist)

	cmd := exec.Command("git", "update-server-info")
	cmd.Dir = repositoryPath
	return cmd.Run()
}

func RPC(user string, gist string, service string) ([]byte, error) {
	repositoryPath := RepositoryPath(user, gist)

	cmd := exec.Command("git", service, "--stateless-rpc", "--advertise-refs", ".")
	cmd.Dir = repositoryPath
	stdout, err := cmd.Output()
	return stdout, err
}

func GcRepos() error {
	subdirs, err := os.ReadDir(filepath.Join(config.GetHomeDir(), ReposDirectory))
	if err != nil {
		return err
	}

	for _, subdir := range subdirs {
		if !subdir.IsDir() {
			continue
		}

		subRoot := filepath.Join(config.GetHomeDir(), ReposDirectory, subdir.Name())

		gitRepos, err := os.ReadDir(subRoot)
		if err != nil {
			log.Warn().Err(err).Msg("Cannot read directory")
			continue
		}

		for _, repo := range gitRepos {
			if !repo.IsDir() {
				continue
			}

			repoPath := filepath.Join(subRoot, repo.Name())

			log.Info().Msg("Running git gc for repository " + repoPath)

			cmd := exec.Command("git", "gc")
			cmd.Dir = repoPath
			err = cmd.Run()
			if err != nil {
				log.Warn().Err(err).Msg("Cannot run git gc for repository " + repoPath)
				continue
			}
		}
	}

	return err
}

func HasNoCommits(user string, gist string) (bool, error) {
	repositoryPath := RepositoryPath(user, gist)

	cmd := exec.Command("git", "rev-parse", "--all")
	cmd.Dir = repositoryPath

	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return false, err
	}

	if out.String() == "" {
		return true, nil // No commits exist
	}

	return false, nil // Commits exist
}

func GetGitVersion() (string, error) {
	cmd := exec.Command("git", "--version")
	stdout, err := cmd.Output()
	if err != nil {
		return "", err
	}

	versionFields := strings.Fields(string(stdout))
	if len(versionFields) < 3 {
		return string(stdout), nil
	}

	return versionFields[2], nil
}

func createDotGitFiles(repositoryPath string) error {
	f1, err := os.OpenFile(filepath.Join(repositoryPath, "git-daemon-export-ok"), os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer f1.Close()

	if err = createDotGitHookFile(repositoryPath, "pre-receive", preReceive); err != nil {
		return err
	}

	return nil
}

func createDotGitHookFile(repositoryPath string, hook string, content string) error {
	preReceiveDst, err := os.OpenFile(filepath.Join(repositoryPath, "hooks", hook), os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0744)
	if err != nil {
		return err
	}

	if _, err = preReceiveDst.WriteString(content); err != nil {
		return err
	}
	defer preReceiveDst.Close()

	return nil
}

func removeFilesExceptGit(dir string) error {
	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && filepath.Base(path) == ".git" {
			return filepath.SkipDir
		}
		if !d.IsDir() {
			return os.Remove(path)
		}
		return nil
	})
}

const preReceive = `#!/bin/sh

disallowed_files=""

while read -r old_rev new_rev ref
do
  if [ "$old_rev" = "0000000000000000000000000000000000000000" ]; then
    # This is the first commit, so we check all the files in that commit
    changed_files=$(git ls-tree -r --name-only "$new_rev")
  else
    # This is not the first commit, so we compare it with its predecessor
    changed_files=$(git diff --name-only "$old_rev" "$new_rev")
  fi

  while IFS= read -r file
  do
    case $file in
      */*)
        disallowed_files="${disallowed_files}${file} "
        ;;
    esac
  done <<EOF
$changed_files
EOF
done

if [ -n "$disallowed_files" ]; then
  echo ""
  echo "Pushing files in folders is not allowed:"
  for file in $disallowed_files; do
    echo "  $file"
  done
  echo ""
  exit 1
fi
`

const postReceive = `#!/bin/sh

while read oldrev newrev refname; do
    if ! git rev-parse --verify --quiet HEAD &>/dev/null; then
        git symbolic-ref HEAD "$refname"
    fi
done

echo ""
echo "Your new repository has been created here: %s"
echo ""
echo "If you want to keep working with your gist, you could set the remote URL via:"
echo "git remote set-url origin %s"
echo ""

rm -f $0
`
