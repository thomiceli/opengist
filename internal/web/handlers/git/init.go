package git

import (
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/web/context"
)

type initKind int

const (
	initNone           initKind = iota // not an /init request
	initStart                          // GET /init/info/refs (create gist, redirect)
	initToken2InfoRefs                 // GET /init/<token>/info/refs
	initToken2Receive                  // POST /init/<token>/git-receive-pack
	initLegacyReceive                  // POST /init/git-receive-pack (no token)
)

// classifyInitRequest inspects the request path and reports which leg of the
// /init push flow it belongs to, returning the correlation token when present.
func classifyInitRequest(urlPath string) (initKind, string) {
	parts := strings.Split(strings.Trim(urlPath, "/"), "/")
	if len(parts) < 2 || parts[0] != "init" {
		return initNone, ""
	}

	switch parts[1] {
	case "info": // /init/info/refs
		return initStart, ""
	case "git-receive-pack": // /init/git-receive-pack (legacy, no token)
		return initLegacyReceive, ""
	default: // /init/<token>/...
		token := parts[1]
		tail := strings.Join(parts[2:], "/")
		switch {
		case strings.HasPrefix(tail, "info/refs"):
			return initToken2InfoRefs, token
		case tail == "git-receive-pack":
			return initToken2Receive, token
		default:
			return initNone, ""
		}
	}
}

// handleInit serves a request belonging to the "git push .../init" flow, after
// the caller has been authenticated.
func handleInit(ctx *context.Context, route *gitRoute, kind initKind, token, username, password string) error {
	user, err := authOrFail(ctx, username, password, db.ScopeGist, db.ReadWritePermission, 401, "Invalid credentials")
	if user == nil {
		return err
	}

	switch kind {
	case initStart:
		return initStartPush(ctx, user)
	case initToken2InfoRefs:
		return initServeToken(ctx, route, user, token, false)
	case initToken2Receive:
		return initServeToken(ctx, route, user, token, true)
	default: // initLegacyReceive
		// git client that did not follow the correlation redirect: fall back to
		// the per-user queue, popping the oldest pending init gist atomically.
		gist, err := db.PopInitGistForUser(user.ID)
		if err != nil {
			return ctx.ErrorRes(500, "Cannot retrieve inited gist from the queue", err)
		}
		setGistContext(ctx, gist)
		return route.handler(ctx)
	}
}

// initStartPush handles the first request of a push to /init: it creates the
// gist and hands the client a per-push token via a redirect. git follows the
// redirect on this initial request (http.followRedirects=initial, the default)
// and uses the redirected URL as the base for the git-receive-pack POST, so that
// POST is unambiguously tied to this gist.
func initStartPush(ctx *context.Context, user *db.User) error {
	gist, err := createGist(user, "")
	if err != nil {
		return ctx.ErrorRes(500, "Cannot create gist", err)
	}

	token, err := newInitToken()
	if err != nil {
		return ctx.ErrorRes(500, "Cannot create init token", err)
	}
	if err = db.AddInitGistToQueue(gist.ID, user.ID, token); err != nil {
		return ctx.ErrorRes(500, "Cannot add inited gist to the queue", err)
	}

	// Relative redirect so it stays correct behind a reverse-proxy subpath.
	noCacheHeaders(ctx)
	return ctx.Redirect(302, "../"+token+"/info/refs?service=git-receive-pack")
}

// initServeToken resolves the gist for a token-carrying second-leg request and
// dispatches to the underlying handler. The info/refs leg looks it up without
// consuming the queue entry; the receive-pack leg atomically consumes it.
func initServeToken(ctx *context.Context, route *gitRoute, user *db.User, token string, consume bool) error {
	var gist *db.Gist
	var err error
	if consume {
		gist, err = db.PopInitGistByToken(token)
	} else {
		gist, err = db.GetInitGistByToken(token)
	}
	if err != nil {
		return ctx.PlainText(404, "Unknown or expired init token")
	}
	if gist.UserID != user.ID {
		log.Warn().Msg("Init token user mismatch from " + ctx.RealIP())
		return ctx.PlainText(404, "Unknown or expired init token")
	}

	setGistContext(ctx, gist)
	return route.handler(ctx)
}

// newInitToken returns a random, URL-safe token used to correlate the two HTTP
// requests of a single push to /init.
func newInitToken() (string, error) {
	u, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}
	return strings.ReplaceAll(u.String(), "-", ""), nil
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
