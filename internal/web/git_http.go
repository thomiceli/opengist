package web

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/thomiceli/opengist/internal/utils"
	"net/http"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/auth"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/git"
	"github.com/thomiceli/opengist/internal/memdb"
	"gorm.io/gorm"
)

var routes = []struct {
	gitUrl  string
	method  string
	handler func(ctx echo.Context) error
}{
	{"(.*?)/git-upload-pack$", "POST", uploadPack},
	{"(.*?)/git-receive-pack$", "POST", receivePack},
	{"(.*?)/info/refs$", "GET", infoRefs},
	{"(.*?)/HEAD$", "GET", textFile},
	{"(.*?)/objects/info/alternates$", "GET", textFile},
	{"(.*?)/objects/info/http-alternates$", "GET", textFile},
	{"(.*?)/objects/info/packs$", "GET", infoPacks},
	{"(.*?)/objects/info/[^/]*$", "GET", textFile},
	{"(.*?)/objects/[0-9a-f]{2}/[0-9a-f]{38}$", "GET", looseObject},
	{"(.*?)/objects/pack/pack-[0-9a-f]{40}\\.pack$", "GET", packFile},
	{"(.*?)/objects/pack/pack-[0-9a-f]{40}\\.idx$", "GET", idxFile},
}

func gitHttp(ctx echo.Context) error {
	for _, route := range routes {
		matched, _ := regexp.MatchString(route.gitUrl, ctx.Request().URL.Path)
		if ctx.Request().Method == route.method && matched {
			if !strings.HasPrefix(ctx.Request().Header.Get("User-Agent"), "git/") {
				continue
			}

			gist := getData(ctx, "gist").(*db.Gist)

			isInit := strings.HasPrefix(ctx.Request().URL.Path, "/init/info/refs")
			isInitReceive := strings.HasPrefix(ctx.Request().URL.Path, "/init/git-receive-pack")
			isInfoRefs := strings.HasSuffix(route.gitUrl, "/info/refs$")
			isPull := ctx.QueryParam("service") == "git-upload-pack" ||
				strings.HasSuffix(ctx.Request().URL.Path, "git-upload-pack") ||
				ctx.Request().Method == "GET" && !isInfoRefs

			repositoryPath := git.RepositoryPath(gist.User.Username, gist.Uuid)
			if _, err := os.Stat(repositoryPath); os.IsNotExist(err) {
				if err != nil {
					log.Info().Err(err).Msg("Repository directory does not exist")
					return errorRes(404, "Repository directory does not exist", err)
				}
			}

			setData(ctx, "repositoryPath", repositoryPath)

			allow, err := auth.ShouldAllowUnauthenticatedGistAccess(ContextAuthInfo{ctx}, true)
			if err != nil {
				log.Fatal().Err(err).Msg("Cannot check if unauthenticated access is allowed")
			}

			// Shows basic auth if :
			// - user wants to push the gist
			// - user wants to clone/pull a private gist
			// - gist is not found (obfuscation)
			// - admin setting to require login is set to true
			if isPull && gist.Private != db.PrivateVisibility && gist.ID != 0 && allow {
				return route.handler(ctx)
			}

			authHeader := ctx.Request().Header.Get("Authorization")
			if authHeader == "" {
				return basicAuth(ctx)
			}

			authFields := strings.Fields(authHeader)
			if len(authFields) != 2 || authFields[0] != "Basic" {
				return basicAuth(ctx)
			}

			authUsername, authPassword, err := basicAuthDecode(authFields[1])
			if err != nil {
				return basicAuth(ctx)
			}

			if !isInit && !isInitReceive {
				if gist.ID == 0 {
					return plainText(ctx, 404, "Check your credentials or make sure you have access to the Gist")
				}

				var userToCheckPermissions *db.User
				if gist.Private != db.PrivateVisibility && isPull {
					userToCheckPermissions, _ = db.GetUserByUsername(authUsername)
				} else {
					userToCheckPermissions = &gist.User
				}

				if ok, err := utils.Argon2id.Verify(authPassword, userToCheckPermissions.Password); !ok {
					if err != nil {
						return errorRes(500, "Cannot verify password", err)
					}
					log.Warn().Msg("Invalid HTTP authentication attempt from " + ctx.RealIP())
					return plainText(ctx, 404, "Check your credentials or make sure you have access to the Gist")
				}
			} else {
				var user *db.User
				if user, err = db.GetUserByUsername(authUsername); err != nil {
					if !errors.Is(err, gorm.ErrRecordNotFound) {
						return errorRes(500, "Cannot get user", err)
					}
					log.Warn().Msg("Invalid HTTP authentication attempt from " + ctx.RealIP())
					return errorRes(401, "Invalid credentials", nil)
				}

				if ok, err := utils.Argon2id.Verify(authPassword, user.Password); !ok {
					if err != nil {
						return errorRes(500, "Cannot check for password", err)
					}
					log.Warn().Msg("Invalid HTTP authentication attempt from " + ctx.RealIP())
					return errorRes(401, "Invalid credentials", nil)
				}

				if isInit {
					gist = new(db.Gist)
					gist.UserID = user.ID
					gist.User = *user
					uuidGist, err := uuid.NewRandom()
					if err != nil {
						return errorRes(500, "Error creating an UUID", err)
					}
					gist.Uuid = strings.Replace(uuidGist.String(), "-", "", -1)
					gist.Title = "gist:" + gist.Uuid

					if err = gist.InitRepository(); err != nil {
						return errorRes(500, "Cannot init repository in the file system", err)
					}

					if err = gist.Create(); err != nil {
						return errorRes(500, "Cannot init repository in database", err)
					}

					if err := memdb.InsertGistInit(user.ID, gist); err != nil {
						return errorRes(500, "Cannot save the URL for the new Gist", err)
					}

					setData(ctx, "gist", gist)
				} else {
					gistFromMemdb, err := memdb.GetGistInitAndDelete(user.ID)
					if err != nil {
						return errorRes(500, "Cannot get the gist link from the in memory database", err)
					}

					gist := gistFromMemdb.Gist
					setData(ctx, "gist", gist)
					setData(ctx, "repositoryPath", git.RepositoryPath(gist.User.Username, gist.Uuid))
				}
			}

			return route.handler(ctx)
		}
	}
	return notFound("Gist not found")
}

func uploadPack(ctx echo.Context) error {
	return pack(ctx, "upload-pack")
}

func receivePack(ctx echo.Context) error {
	return pack(ctx, "receive-pack")
}

func pack(ctx echo.Context, serviceType string) error {
	noCacheHeaders(ctx)
	defer ctx.Request().Body.Close()

	if ctx.Request().Header.Get("Content-Type") != "application/x-git-"+serviceType+"-request" {
		return errorRes(401, "Git client unsupported", nil)
	}
	ctx.Response().Header().Set("Content-Type", "application/x-git-"+serviceType+"-result")

	var err error
	reqBody := ctx.Request().Body

	if ctx.Request().Header.Get("Content-Encoding") == "gzip" {
		reqBody, err = gzip.NewReader(reqBody)
		if err != nil {
			return errorRes(500, "Cannot create gzip reader", err)
		}
	}

	repositoryPath := getData(ctx, "repositoryPath").(string)
	gist := getData(ctx, "gist").(*db.Gist)

	var stderr bytes.Buffer
	cmd := exec.Command("git", serviceType, "--stateless-rpc", repositoryPath)
	cmd.Dir = repositoryPath
	cmd.Stdin = reqBody
	cmd.Stdout = ctx.Response().Writer
	cmd.Stderr = &stderr
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "OPENGIST_REPOSITORY_URL_INTERNAL="+git.RepositoryUrl(ctx, gist.User.Username, gist.Identifier()))
	cmd.Env = append(cmd.Env, "OPENGIST_REPOSITORY_ID="+strconv.Itoa(int(gist.ID)))

	if err = cmd.Run(); err != nil {
		return errorRes(500, "Cannot run git "+serviceType+" ; "+stderr.String(), err)
	}

	return nil
}

func infoRefs(ctx echo.Context) error {
	noCacheHeaders(ctx)
	var service string

	gist := getData(ctx, "gist").(*db.Gist)

	serviceType := ctx.QueryParam("service")
	if strings.HasPrefix(serviceType, "git-") {
		service = strings.TrimPrefix(serviceType, "git-")
	}

	if service != "upload-pack" && service != "receive-pack" {
		if err := gist.UpdateServerInfo(); err != nil {
			return errorRes(500, "Cannot update server info", err)
		}
		return sendFile(ctx, "text/plain; charset=utf-8")
	}

	refs, err := gist.RPC(service)
	if err != nil {
		return errorRes(500, "Cannot run git "+service, err)
	}

	ctx.Response().Header().Set("Content-Type", "application/x-git-"+service+"-advertisement")
	ctx.Response().WriteHeader(200)
	_, _ = ctx.Response().Write(packetWrite("# service=git-" + service + "\n"))
	_, _ = ctx.Response().Write([]byte("0000"))
	_, _ = ctx.Response().Write(refs)

	return nil
}

func textFile(ctx echo.Context) error {
	noCacheHeaders(ctx)
	return sendFile(ctx, "text/plain")
}

func infoPacks(ctx echo.Context) error {
	cacheHeadersForever(ctx)
	return sendFile(ctx, "text/plain; charset=utf-8")
}

func looseObject(ctx echo.Context) error {
	cacheHeadersForever(ctx)
	return sendFile(ctx, "application/x-git-loose-object")
}

func packFile(ctx echo.Context) error {
	cacheHeadersForever(ctx)
	return sendFile(ctx, "application/x-git-packed-objects")
}

func idxFile(ctx echo.Context) error {
	cacheHeadersForever(ctx)
	return sendFile(ctx, "application/x-git-packed-objects-toc")
}

func noCacheHeaders(ctx echo.Context) {
	ctx.Response().Header().Set("Expires", "Thu, 01 Jan 1970 00:00:00 UTC")
	ctx.Response().Header().Set("Pragma", "no-cache")
	ctx.Response().Header().Set("Cache-Control", "no-cache, max-age=0, must-revalidate")
}

func cacheHeadersForever(ctx echo.Context) {
	now := time.Now().Unix()
	expires := now + 31536000
	ctx.Response().Header().Set("Date", fmt.Sprintf("%d", now))
	ctx.Response().Header().Set("Expires", fmt.Sprintf("%d", expires))
	ctx.Response().Header().Set("Cache-Control", "public, max-age=31536000")
}

func basicAuth(ctx echo.Context) error {
	ctx.Response().Header().Set("WWW-Authenticate", `Basic realm="."`)
	return plainText(ctx, 401, "Requires authentication")
}

func basicAuthDecode(encoded string) (string, string, error) {
	s, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", "", err
	}

	auth := strings.SplitN(string(s), ":", 2)
	return auth[0], auth[1], nil
}

func sendFile(ctx echo.Context, contentType string) error {
	gitFile := "/" + strings.Join(strings.Split(ctx.Request().URL.Path, "/")[3:], "/")
	gitFile = path.Join(getData(ctx, "repositoryPath").(string), gitFile)
	fi, err := os.Stat(gitFile)
	if os.IsNotExist(err) {
		return errorRes(404, "File not found", nil)
	}
	ctx.Response().Header().Set("Content-Type", contentType)
	ctx.Response().Header().Set("Content-Length", fmt.Sprintf("%d", fi.Size()))
	ctx.Response().Header().Set("Last-Modified", fi.ModTime().Format(http.TimeFormat))
	return ctx.File(gitFile)
}

func packetWrite(str string) []byte {
	s := strconv.FormatInt(int64(len(str)+4), 16)

	if len(s)%4 != 0 {
		s = strings.Repeat("0", 4-len(s)%4) + s
	}

	return []byte(s + str)
}
