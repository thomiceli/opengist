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

	gist := ctx.Get("gist").(*db.Gist)
	gistExists := gist.ID != 0

	// Expired gists are removed by a background job, but it may not have run
	// yet: handle them as gists that don't exist, except that handlePush won't
	// create a new gist in place of one.
	if gist.IsExpired() {
		gistExists = false
	}

	initKind, initToken := classifyInitRequest(ctx.Request().URL.Path)

	// The matched route decides whether the request is a pull or a push, so it
	// is always authorized for the handler that will serve it. Only info/refs
	// names the service in the query string; git never sends that parameter
	// elsewhere, so it is refused rather than allowed to disagree with the route.
	service := ctx.QueryParam("service")
	isInfoRefs := strings.HasSuffix(route.gitUrl, "/info/refs$")
	if service != "" && !isInfoRefs {
		return ctx.PlainText(400, "Invalid service parameter")
	}
	isPull := strings.HasSuffix(route.gitUrl, "/git-upload-pack$") || isInfoRefs && service == "git-upload-pack"
	isPush := strings.HasSuffix(route.gitUrl, "/git-receive-pack$") || isInfoRefs && service == "git-receive-pack"

	// Anything that is neither an init request, a smart pull, nor a push is a
	// dumb-protocol read: GET .../info/refs (no service), .../HEAD,
	// .../objects/*. These stream repository files straight off disk, so they
	// are read operations equivalent to a pull and must be authorized as one —
	// never served merely because an Authorization header is present.
	isDumb := initKind == initNone && !isPull && !isPush

	ctx.SetData("repositoryPath", git.RepositoryPath(gist.User.Username, gist.Uuid))

	allow, err := auth.ShouldAllowUnauthenticatedGistAccess(handlers.ContextAuthInfo{Context: ctx}, true)
	if err != nil {
		log.Fatal().Err(err).Msg("Cannot check if unauthenticated access is allowed")
	}

	// No need to authenticate if the user wants to clone/pull (smart or dumb) ;
	// a non-private gist ; that exists ; where unauthenticated access is allowed
	// in the instance
	if (isPull || isDumb) && gist.Private != db.PrivateVisibility && gistExists && allow {
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
	case isPull || isDumb:
		return handlePull(ctx, route, gist, gistExists, authUsername, authPassword)
	case isPush:
		return handlePush(ctx, route, gist, gistExists, authUsername, authPassword)
	default:
		return ctx.NotFound("Gist not found")
	}
}

// setGistContext points the request at a specific gist and its repository path,
// overriding whatever the soft-init middleware put in place.
func setGistContext(ctx *context.Context, gist *db.Gist) {
	ctx.Set("gist", gist)
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
