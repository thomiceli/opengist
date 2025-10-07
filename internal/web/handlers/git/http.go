package git

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/auth"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/git"
	"github.com/thomiceli/opengist/internal/web/context"
	"github.com/thomiceli/opengist/internal/web/handlers"
)

var routes = []struct {
	gitUrl  string
	method  string
	handler func(ctx *context.Context) error
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

func GitHttp(ctx *context.Context) error {
	route := findMatchingRoute(ctx)
	if route == nil {
		return ctx.NotFound("Gist not found") // regular 404 for non-git routes
	}
	gist := ctx.GetData("gist").(*db.Gist)
	gistExists := gist.ID != 0

	isInitRoute := strings.HasPrefix(ctx.Request().URL.Path, "/init/info/refs")
	isInitRouteReceive := strings.HasPrefix(ctx.Request().URL.Path, "/init/git-receive-pack")
	isInfoRefs := strings.HasSuffix(route.gitUrl, "/info/refs$")
	isPull := ctx.QueryParam("service") == "git-upload-pack" ||
		strings.HasSuffix(ctx.Request().URL.Path, "git-upload-pack") && !isInfoRefs
	isPush := ctx.QueryParam("service") == "git-receive-pack" ||
		strings.HasSuffix(ctx.Request().URL.Path, "git-receive-pack") && !isInfoRefs

	repositoryPath := git.RepositoryPath(gist.User.Username, gist.Uuid)
	ctx.SetData("repositoryPath", repositoryPath)

	allow, err := auth.ShouldAllowUnauthenticatedGistAccess(handlers.ContextAuthInfo{Context: ctx}, true)
	if err != nil {
		log.Fatal().Err(err).Msg("Cannot check if unauthenticated access is allowed")
	}

	// No need to authenticate if the user wants
	// to clone/pull ; a non-private gist ; that exists ; where unauthenticated access is allowed in the instance
	if isPull && gist.Private != db.PrivateVisibility && gistExists && allow {
		return route.handler(ctx)
	}

	// Else we need to authenticate the user, that include other cases:
	// - user wants to push the gist
	// - user wants to clone/pull a private gist
	// - user wants to clone/pull a non-private gist but unauthenticated access is not allowed
	// - gist is not found ; has no right to clone/pull (obfuscation)
	// - admin setting to require login is set to true
	authUsername, authPassword, err := parseAuthHeader(ctx)
	if err != nil {
		return basicAuth(ctx)
	}

	// if the user wants to create a gist via the /init route
	if isInitRoute || isInitRouteReceive {
		var user *db.User

		// check if the user has a valid account on opengist to push a gist
		user, err = auth.TryAuthentication(authUsername, authPassword)
		if err != nil {
			var authErr auth.AuthError
			if errors.As(err, &authErr) {
				log.Warn().Msg("Invalid HTTP authentication attempt from " + ctx.RealIP())
				return ctx.PlainText(401, "Invalid credentials")
			}
			return ctx.ErrorRes(500, "Authentication system error", nil)
		}

		if isInitRoute {
			gist, err = createGist(user, "")
			if err != nil {
				return ctx.ErrorRes(500, "Cannot create gist", err)
			}

			err = db.AddInitGistToQueue(gist.ID, user.ID)
			if err != nil {
				return ctx.ErrorRes(500, "Cannot add inited gist to the queue", err)
			}
			ctx.SetData("gist", gist)
			return route.handler(ctx)
		} else {
			gist, err = db.GetInitGistInQueueForUser(user.ID)
			if err != nil {
				return ctx.ErrorRes(500, "Cannot retrieve inited gist from the queue", err)
			}

			ctx.SetData("gist", gist)
			ctx.SetData("repositoryPath", git.RepositoryPath(gist.User.Username, gist.Uuid))
			return route.handler(ctx)
		}
	}

	// if clone/pull
	// check if the gist exists and if the credentials are valid
	if isPull {
		log.Debug().Msg("Detected git pull operation")
		if !gistExists {
			log.Debug().Str("authUsername", authUsername).Msg("Pulling unknown gist")
			return ctx.PlainText(404, "Check your credentials or make sure you have access to the Gist")
		}

		var userToCheckPermissions string
		// if the user is trying to clone/pull a non-private gist while unauthenticated access is not allowed,
		// check if the user has a valid account
		if gist.Private != db.PrivateVisibility {
			log.Debug().Str("authUsername", authUsername).Msg("Pulling non-private gist with authenticated access")
			userToCheckPermissions = authUsername
		} else { // else just check the password against the gist owner
			log.Debug().Str("authUsername", authUsername).Str("gistOwner", gist.User.Username).Msg("Pulling private gist")
			userToCheckPermissions = gist.User.Username
		}

		if _, err = auth.TryAuthentication(userToCheckPermissions, authPassword); err != nil {
			var authErr auth.AuthError
			if errors.As(err, &authErr) {
				log.Warn().Msg("Invalid HTTP authentication attempt from " + ctx.RealIP())
				return ctx.PlainText(404, "Check your credentials or make sure you have access to the Gist")
			}
			return ctx.ErrorRes(500, "Authentication system error", nil)
		}
		log.Debug().Str("authUsername", authUsername).Msg("Pulling gist")

		return route.handler(ctx)
	}

	if isPush {
		log.Debug().Msg("Detected git push operation")
		// if gist exists, check if the credentials are valid and if the user is the gist owner
		if gistExists {
			log.Debug().Str("authUsername", authUsername).Str("gistOwner", gist.User.Username).Msg("Pushing to existing gist")
			if _, err = auth.TryAuthentication(gist.User.Username, authPassword); err != nil {
				var authErr auth.AuthError
				if errors.As(err, &authErr) {
					log.Warn().Msg("Invalid HTTP authentication attempt from " + ctx.RealIP())
					return ctx.PlainText(404, "Check your credentials or make sure you have access to the Gist")
				}
				return ctx.ErrorRes(500, "Authentication system error", nil)
			}
			log.Debug().Str("authUsername", authUsername).Msg("Pushing gist")

			return route.handler(ctx)
		} else { // if the gist does not exist, check if the user has a valid account on opengist to push a gist and create it
			log.Debug().Str("authUsername", authUsername).Msg("Creating new gist by pushing")
			var user *db.User
			if user, err = auth.TryAuthentication(authUsername, authPassword); err != nil {
				var authErr auth.AuthError
				if errors.As(err, &authErr) {
					log.Warn().Msg("Invalid HTTP authentication attempt from " + ctx.RealIP())
					return ctx.PlainText(404, "Check your credentials or make sure you have access to the Gist")
				}
				return ctx.ErrorRes(500, "Authentication system error", nil)
			}

			urlPath := ctx.Request().URL.Path
			pathParts := strings.Split(strings.Trim(urlPath, "/"), "/")
			if pathParts[0] == authUsername && len(pathParts) == 4 {
				log.Debug().Str("authUsername", authUsername).Msg("Valid URL format for push operation")
				gist, err = createGist(user, pathParts[1])
				if err != nil {
					return ctx.ErrorRes(500, "Cannot create gist", err)
				}
				log.Debug().Str("authUsername", authUsername).Str("url", urlPath).Msg("Gist created")
				ctx.SetData("gist", gist)
				ctx.SetData("repositoryPath", git.RepositoryPath(gist.User.Username, gist.Uuid))
			} else {
				log.Debug().Str("authUsername", authUsername).Any("path", pathParts).Msg("Invalid URL format for push operation")
				return ctx.PlainText(401, "Invalid URL format for push operation")
			}
			return route.handler(ctx)

		}
	}

	return route.handler(ctx)
}

func findMatchingRoute(ctx *context.Context) *struct {
	gitUrl  string
	method  string
	handler func(ctx *context.Context) error
} {
	for _, route := range routes {
		matched, _ := regexp.MatchString(route.gitUrl, ctx.Request().URL.Path)
		if ctx.Request().Method == route.method && matched {
			if !strings.HasPrefix(ctx.Request().Header.Get("User-Agent"), "git/") {
				continue
			}
			return &route
		}
	}
	return nil
}

func createGist(user *db.User, url string) (*db.Gist, error) {
	gist := new(db.Gist)
	gist.UserID = user.ID
	gist.User = *user
	uuidGist, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}
	gist.Uuid = strings.ReplaceAll(uuidGist.String(), "-", "")
	gist.Title = "gist:" + gist.Uuid

	if url != "" {
		gist.URL = strings.TrimSuffix(url, ".git")
		gist.Title = strings.TrimSuffix(url, ".git")
	}

	if err := gist.InitRepository(); err != nil {
		return nil, err
	}

	if err := gist.Create(); err != nil {
		return nil, err
	}

	return gist, nil
}

func uploadPack(ctx *context.Context) error {
	return pack(ctx, "upload-pack")
}

func receivePack(ctx *context.Context) error {
	return pack(ctx, "receive-pack")
}

func pack(ctx *context.Context, serviceType string) error {
	noCacheHeaders(ctx)
	defer ctx.Request().Body.Close()

	if ctx.Request().Header.Get("Content-Type") != "application/x-git-"+serviceType+"-request" {
		return ctx.ErrorRes(401, "Git client unsupported", nil)
	}
	ctx.Response().Header().Set("Content-Type", "application/x-git-"+serviceType+"-result")

	var err error
	reqBody := ctx.Request().Body

	if ctx.Request().Header.Get("Content-Encoding") == "gzip" {
		reqBody, err = gzip.NewReader(reqBody)
		if err != nil {
			return ctx.ErrorRes(500, "Cannot create gzip reader", err)
		}
	}

	repositoryPath := ctx.GetData("repositoryPath").(string)
	gist := ctx.GetData("gist").(*db.Gist)

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
		return ctx.ErrorRes(500, "Cannot run git "+serviceType+" ; "+stderr.String(), err)
	}

	return nil
}

func infoRefs(ctx *context.Context) error {
	noCacheHeaders(ctx)
	var service string

	gist := ctx.GetData("gist").(*db.Gist)

	serviceType := ctx.QueryParam("service")
	if strings.HasPrefix(serviceType, "git-") {
		service = strings.TrimPrefix(serviceType, "git-")
	}

	if service != "upload-pack" && service != "receive-pack" {
		if err := gist.UpdateServerInfo(); err != nil {
			return ctx.ErrorRes(500, "Cannot update server info", err)
		}
		return sendFile(ctx, "text/plain; charset=utf-8")
	}

	refs, err := gist.RPC(service)
	if err != nil {
		return ctx.ErrorRes(500, "Cannot run git "+service, err)
	}

	ctx.Response().Header().Set("Content-Type", "application/x-git-"+service+"-advertisement")
	ctx.Response().WriteHeader(200)
	_, _ = ctx.Response().Write(packetWrite("# service=git-" + service + "\n"))
	_, _ = ctx.Response().Write([]byte("0000"))
	_, _ = ctx.Response().Write(refs)

	return nil
}

func textFile(ctx *context.Context) error {
	noCacheHeaders(ctx)
	return sendFile(ctx, "text/plain")
}

func infoPacks(ctx *context.Context) error {
	cacheHeadersForever(ctx)
	return sendFile(ctx, "text/plain; charset=utf-8")
}

func looseObject(ctx *context.Context) error {
	cacheHeadersForever(ctx)
	return sendFile(ctx, "application/x-git-loose-object")
}

func packFile(ctx *context.Context) error {
	cacheHeadersForever(ctx)
	return sendFile(ctx, "application/x-git-packed-objects")
}

func idxFile(ctx *context.Context) error {
	cacheHeadersForever(ctx)
	return sendFile(ctx, "application/x-git-packed-objects-toc")
}

func noCacheHeaders(ctx *context.Context) {
	ctx.Response().Header().Set("Expires", "Thu, 01 Jan 1970 00:00:00 UTC")
	ctx.Response().Header().Set("Pragma", "no-cache")
	ctx.Response().Header().Set("Cache-Control", "no-cache, max-age=0, must-revalidate")
}

func cacheHeadersForever(ctx *context.Context) {
	now := time.Now().Unix()
	expires := now + 31536000
	ctx.Response().Header().Set("Date", fmt.Sprintf("%d", now))
	ctx.Response().Header().Set("Expires", fmt.Sprintf("%d", expires))
	ctx.Response().Header().Set("Cache-Control", "public, max-age=31536000")
}

func basicAuth(ctx *context.Context) error {
	ctx.Response().Header().Set("WWW-Authenticate", `Basic realm="."`)
	return ctx.PlainText(401, "Requires authentication")
}

func parseAuthHeader(ctx *context.Context) (string, string, error) {
	authHeader := ctx.Request().Header.Get("Authorization")
	if authHeader == "" {
		return "", "", errors.New("no auth header")
	}

	authFields := strings.Fields(authHeader)
	if len(authFields) != 2 || authFields[0] != "Basic" {
		return "", "", errors.New("invalid auth header")
	}

	authUsername, authPassword, err := basicAuthDecode(authFields[1])
	if err != nil {
		log.Error().Err(err).Msg("Cannot decode basic auth header")
		return "", "", err
	}

	return authUsername, authPassword, nil
}

func basicAuthDecode(encoded string) (string, string, error) {
	s, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", "", err
	}

	auth := strings.SplitN(string(s), ":", 2)
	return auth[0], auth[1], nil
}

func sendFile(ctx *context.Context, contentType string) error {
	gitFile := "/" + strings.Join(strings.Split(ctx.Request().URL.Path, "/")[3:], "/")
	gitFile = path.Join(ctx.GetData("repositoryPath").(string), gitFile)
	fi, err := os.Stat(gitFile)
	if os.IsNotExist(err) {
		return ctx.ErrorRes(404, "File not found", nil)
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
