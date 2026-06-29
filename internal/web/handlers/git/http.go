package git

import (
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/auth"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/git"
	"github.com/thomiceli/opengist/internal/web/context"
	"github.com/thomiceli/opengist/internal/web/handlers"
)

// gitRoute maps a Git smart/dumb HTTP URL (matched as a regexp) and method to
// the handler that serves it.
type gitRoute struct {
	gitUrl  string
	method  string
	handler func(ctx *context.Context) error
}

var routes = []gitRoute{
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

// GitHttp is the entry point for all Git-over-HTTP requests. It resolves the
// matching route, classifies the request (init / pull / push) and dispatches to
// the relevant handler once access has been authorized.
func GitHttp(ctx *context.Context) error {
	route := findMatchingRoute(ctx)
	if route == nil {
		return ctx.NotFound("Gist not found") // regular 404 for non-git routes
	}

	gist := ctx.GetData("gist").(*db.Gist)
	gistExists := gist.ID != 0

	initKind, initToken := classifyInitRequest(ctx.Request().URL.Path)
	isInfoRefs := strings.HasSuffix(route.gitUrl, "/info/refs$")
	isPull := ctx.QueryParam("service") == "git-upload-pack" ||
		strings.HasSuffix(ctx.Request().URL.Path, "git-upload-pack") && !isInfoRefs
	isPush := ctx.QueryParam("service") == "git-receive-pack" ||
		strings.HasSuffix(ctx.Request().URL.Path, "git-receive-pack") && !isInfoRefs

	ctx.SetData("repositoryPath", git.RepositoryPath(gist.User.Username, gist.Uuid))

	allow, err := auth.ShouldAllowUnauthenticatedGistAccess(handlers.ContextAuthInfo{Context: ctx}, true)
	if err != nil {
		log.Fatal().Err(err).Msg("Cannot check if unauthenticated access is allowed")
	}

	// No need to authenticate if the user wants to clone/pull ; a non-private
	// gist ; that exists ; where unauthenticated access is allowed in the instance
	if isPull && gist.Private != db.PrivateVisibility && gistExists && allow {
		return route.handler(ctx)
	}

	// Every other case needs credentials:
	// - user wants to push the gist
	// - user wants to clone/pull a private gist
	// - user wants to clone/pull a non-private gist but unauthenticated access is not allowed
	// - gist is not found ; has no right to clone/pull (obfuscation)
	// - admin setting to require login is set to true
	authUsername, authPassword, err := parseAuthHeader(ctx)
	if err != nil {
		return basicAuth(ctx)
	}

	switch {
	case initKind != initNone:
		return handleInit(ctx, route, initKind, initToken, authUsername, authPassword)
	case isPull:
		return handlePull(ctx, route, gist, gistExists, authUsername, authPassword)
	case isPush:
		return handlePush(ctx, route, gist, gistExists, authUsername, authPassword)
	default:
		return route.handler(ctx)
	}
}

// setGistContext points the request at a specific gist and its repository path,
// overriding whatever the soft-init middleware put in place.
func setGistContext(ctx *context.Context, gist *db.Gist) {
	ctx.SetData("gist", gist)
	ctx.SetData("repositoryPath", git.RepositoryPath(gist.User.Username, gist.Uuid))
}

func findMatchingRoute(ctx *context.Context) *gitRoute {
	for i := range routes {
		route := &routes[i]
		matched, _ := regexp.MatchString(route.gitUrl, ctx.Request().URL.Path)
		if ctx.Request().Method == route.method && matched {
			if !strings.HasPrefix(ctx.Request().Header.Get("User-Agent"), "git/") {
				continue
			}
			return route
		}
	}
	return nil
}
