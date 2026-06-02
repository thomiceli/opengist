package v1

import (
	"net/url"

	"github.com/thomiceli/opengist/internal/web/context"
)

// RawFile handles GET /api/v1/gists/:uuid/files/:sha/:filename.
// Returns the raw bytes of `filename` as committed at `sha`. Visibility
// rules mirror GetGist/GetGistRevision:
//
//   - Public/unlisted gists readable by anyone (anonymous OK).
//   - Private gists only by their owner with a gist:read token.
//
// Error codes split the failure modes for the client:
//   - 404 "Gist not found"     - gist doesn't exist or caller can't see it.
//   - 400 "Invalid revision format" - :sha isn't pure hex (also guards
//     against argv injection into the underlying git command).
//   - 404 "File not found"     - sha is fine but the file (or the revision)
//     doesn't resolve in the repo.
//
// The body is the file's content, with Content-Type / Content-Disposition
// derived from the detected mime type. SVG and PDF responses also carry a
// restrictive Content-Security-Policy header, matching the web RawFile so
// embedding hostile content can't break out into script execution.
func RawFile(ctx *context.Context) error {
	g, err := lookupGistByUUID(ctx, ctx.Param("uuid"))
	if err != nil {
		return ctx.ErrorJson(404, "Gist not found", nil)
	}

	sha := ctx.Param("sha")
	if !reCommitSHA.MatchString(sha) {
		return ctx.ErrorJson(400, "Invalid revision format", nil)
	}

	file, err := g.File(sha, ctx.Param("filename"), false)
	if err != nil {
		return ctx.ErrorJson(500, "failed to read file", err)
	}
	if file == nil {
		return ctx.ErrorJson(404, "File not found", nil)
	}

	// Mirror the web RawFile's security headers for SVG/PDF - these formats
	// can carry script that runs when rendered inline.
	if file.MimeType.IsSVG() {
		ctx.Response().Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	} else if file.MimeType.IsPDF() {
		ctx.Response().Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
	}

	switch {
	case file.MimeType.CanBeEmbedded():
		ctx.Response().Header().Set("Content-Type", file.MimeType.ContentType)
	case file.MimeType.IsText():
		ctx.Response().Header().Set("Content-Type", "text/plain; charset=utf-8")
	default:
		ctx.Response().Header().Set("Content-Type", "application/octet-stream")
	}

	ctx.Response().Header().Set("Content-Disposition",
		"inline; filename=\""+url.PathEscape(file.Filename)+"\"")
	ctx.Response().Header().Set("X-Content-Type-Options", "nosniff")
	return ctx.PlainText(200, file.Content)
}
