package v1

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/git"
	"github.com/thomiceli/opengist/internal/web/context"
	"github.com/thomiceli/opengist/internal/web/handlers/api/v1/types"
)

// strOrEmpty dereferences an optional string field, returning "" when nil.
// Handy for the CREATE flow where "no value provided" and "explicit empty
// string" land on the same DB column.
func strOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// CreateGist handles POST /api/gists.
// The DTO is built the same way ProcessCreate builds its form DTO - entries
// without `content` are skipped, empty filenames become "gistfileN.txt" -
// and is then run through ctx.Validate so the API and the web form share
// rules (length caps on title/description/filename, forbidden chars in
// filenames, `min=1` on files). Returns 201 with the full gist and a
// `Location` header on success; validation errors → 422.
func CreateGist(ctx *context.Context) error {
	var req types.GistInput
	if err := ctx.Bind(&req); err != nil {
		return ctx.ErrorJson(422, "could not bind data", nil)
	}

	// Sort filenames so the Title fallback (first filename) and auto-generated
	// "gistfileN.txt" names are deterministic.
	filenames := make([]string, 0, len(req.Files))
	for name := range req.Files {
		filenames = append(filenames, name)
	}
	sort.Strings(filenames)

	dto := &db.GistDTO{
		Title:         strOrEmpty(req.Title),
		Description:   strOrEmpty(req.Description),
		Expire:        db.ExpirationType(strOrEmpty(req.Expire)),
		VisibilityDTO: db.VisibilityDTO{Private: db.ParseVisibility(strOrEmpty(req.Visibility))},
	}
	// An explicit custom date takes precedence over the preset.
	if req.ExpiresAt != nil {
		dto.Expire = db.ExpiryCustom
		dto.ExpireAt = *req.ExpiresAt
	}
	for _, rawName := range filenames {
		f := req.Files[rawName]
		if f == nil || f.Content == nil || *f.Content == "" {
			// Matches ProcessCreate: entries without content are silently
			// dropped. min=1 on Files then catches "no files at all".
			continue
		}
		// On create the map key is the filename; the per-entry `filename`
		// field is ignored here (it only matters on update, for renames).
		name := git.CleanTreePathName(rawName)
		if name == "" {
			name = "gistfile" + strconv.Itoa(len(dto.Files)+1) + ".txt"
		}
		dto.Files = append(dto.Files, db.FileDTO{
			Filename: strings.TrimSpace(name),
			Content:  *f.Content,
		})
	}

	if err := ctx.Validate(dto); err != nil {
		return ctx.ErrorJson(422, err.Error(), nil)
	}

	user := ctx.User
	gist := dto.ToGist()
	gist.UserID = user.ID
	gist.User = *user
	gist.NbFiles = len(dto.Files)

	gist.ExpiresAt = dto.ExpiresAtTimestamp()

	id, err := uuid.NewRandom()
	if err != nil {
		return ctx.ErrorJson(500, "uuid generation failed", err)
	}
	gist.Uuid = strings.ReplaceAll(id.String(), "-", "")
	if gist.Title == "" {
		if dto.Files[0].Filename == "" {
			gist.Title = "gist:" + gist.Uuid
		} else {
			gist.Title = dto.Files[0].Filename
		}
	}

	if err := gist.InitRepository(); err != nil {
		return ctx.ErrorJson(500, "failed to init repo", err)
	}
	if err := gist.AddAndCommitFiles(&dto.Files); err != nil {
		gist.DeleteRepository()
		return ctx.ErrorJson(500, "failed to commit files", err)
	}
	if err := gist.Create(); err != nil {
		gist.DeleteRepository()
		return ctx.ErrorJson(500, "failed to create gist", err)
	}
	gist.AddInIndex()
	gist.UpdateLanguages()
	_ = gist.UpdatePreviewAndCount(true)

	saved, err := db.GetGistByID(strconv.FormatUint(uint64(gist.ID), 10))
	if err != nil {
		return ctx.ErrorJson(500, "failed to reload gist", err)
	}
	baseURL := apiBaseURL(ctx)
	resp, err := saved.ToAPI(baseURL, "HEAD")
	if err != nil {
		return ctx.ErrorJson(500, "failed to serialize gist", err)
	}
	ctx.Response().Header().Set("Location", baseURL+"/api/gists/"+saved.Uuid)
	return ctx.JSON(201, resp)
}

// UpdateGist handles PATCH /api/gists/:uuid.
// Only fields present in the body are touched. Files not mentioned in `files`
// stay unchanged. A file entry
// can:
//
//   - Set `content` to replace the file body.
//   - Set `filename` to rename the file.
//   - Set both to do both at once.
//   - Be JSON null (or have neither field set) to delete the file.
//
// Keys in the `files` map that don't match any current filename are treated
// as new files; their `content` is required. `title` and `visibility` are
// Opengist extensions - supplying them updates those fields, omission leaves
// them alone. Returns the full updated gist on success; 422 on validation
// failure, 404 if the caller can't see the gist.
func UpdateGist(ctx *context.Context) error {
	g, err := lookupGistByUUID(ctx, ctx.Param("uuid"))
	if err != nil {
		// Lookup failure already obeys the "hide existence" rule for private
		// gists, so we just propagate 404.
		return ctx.ErrorJson(404, "Gist not found", nil)
	}
	if g.UserID != ctx.User.ID {
		// At this point the gist is public or unlisted (private would have
		// 404'd above) - existence is already disclosed, so a 403 is honest.
		return ctx.ErrorJson(403, "You are not the owner of this gist", nil)
	}
	if g.Archived {
		return ctx.ErrorJson(403, "This gist is archived and is read-only", nil)
	}

	var req types.GistInput
	if err := ctx.Bind(&req); err != nil {
		return ctx.ErrorJson(422, "could not bind data", nil)
	}

	// PATCH requires at least one actionable field - otherwise we'd just
	// rewrite the gist's updated_at for no reason.
	if req.Description == nil && req.Title == nil && req.Visibility == nil && len(req.Files) == 0 {
		return ctx.ErrorJson(422, "at least one of description, title, visibility, or files must be set", nil)
	}

	if req.Title != nil {
		g.Title = *req.Title
	}
	if req.Description != nil {
		g.Description = *req.Description
	}
	if req.Visibility != nil {
		g.Private = db.ParseVisibility(*req.Visibility)
	}

	// File patch: only rebuild the working tree if `files` carried at least
	// one entry. (`files: {}` is a no-op.)
	if len(req.Files) > 0 {
		merged, err := mergePatchFiles(g, req.Files)
		if err != nil {
			return ctx.ErrorJson(422, err.Error(), nil)
		}

		dto := &db.GistDTO{
			Title:         g.Title,
			Description:   g.Description,
			VisibilityDTO: db.VisibilityDTO{Private: g.Private},
			Files:         merged,
		}
		if err := ctx.Validate(dto); err != nil {
			return ctx.ErrorJson(422, err.Error(), nil)
		}

		g.NbFiles = len(dto.Files)
		if err := g.AddAndCommitFiles(&dto.Files); err != nil {
			return ctx.ErrorJson(500, "failed to commit files", err)
		}
	}

	if err := g.Update(); err != nil {
		return ctx.ErrorJson(500, "failed to update gist", err)
	}
	g.UpdateLanguages()
	_ = g.UpdatePreviewAndCount(true)

	resp, err := g.ToAPI(apiBaseURL(ctx), "HEAD")
	if err != nil {
		return ctx.ErrorJson(500, "failed to serialize gist", err)
	}
	return ctx.JSON(200, resp)
}

// DeleteGist handles DELETE /api/gists/:uuid.
// Owner-only - the route's apiScope(ScopeGist, ReadWritePermission) middleware
// enforces the token scope before we get here, so we just confirm ownership and
// drop the repo + row. Returns 204 No Content on success.
func DeleteGist(ctx *context.Context) error {
	g, err := lookupGistByUUID(ctx, ctx.Param("uuid"))
	if err != nil {
		return ctx.ErrorJson(404, "Gist not found", nil)
	}
	if g.UserID != ctx.User.ID {
		return ctx.ErrorJson(403, "You are not the owner of this gist", nil)
	}
	// db.Gist.Delete deletes the repo first, then the DB row - no need to
	// call DeleteRepository ourselves.
	if err := g.Delete(); err != nil {
		return ctx.ErrorJson(500, "failed to delete gist", err)
	}
	return ctx.NoContent(204)
}

// mergePatchFiles applies a PATCH file map to a gist's current files and
// returns the post-merge file list. Behavior per entry:
//
//   - Key matches an existing filename:
//   - patch == nil OR (Content == nil AND Filename == nil) → delete.
//   - patch.Filename set → rename (Content unchanged unless also set).
//   - patch.Content  set → update content (Filename unchanged unless set).
//   - Key doesn't match any existing filename:
//   - patch.Content set → add as a new file.
//   - otherwise → no-op (null on an unknown key just does nothing instead
//     of erroring).
//
// Detects post-merge filename collisions and returns an error rather than
// letting the second write silently overwrite the first.
func mergePatchFiles(g *db.Gist, patch map[string]*types.GistFileInput) ([]db.FileDTO, error) {
	// Full content, no truncation - we have to round-trip everything.
	current, _, err := g.Files("HEAD", false)
	if err != nil {
		return nil, fmt.Errorf("failed to read current files")
	}

	merged := make([]db.FileDTO, 0, len(current))
	handled := make(map[string]bool, len(patch))

	for _, cf := range current {
		entry, mentioned := patch[cf.Filename]
		if mentioned {
			handled[cf.Filename] = true
			// Delete: explicit null, or no content + no filename change.
			if entry == nil || (entry.Content == nil && entry.Filename == nil) {
				continue
			}
			name := cf.Filename
			if entry.Filename != nil {
				name = *entry.Filename
			}
			content := cf.Content
			if entry.Content != nil {
				content = *entry.Content
			}
			merged = append(merged, db.FileDTO{Filename: name, Content: content})
			continue
		}
		merged = append(merged, db.FileDTO{Filename: cf.Filename, Content: cf.Content})
	}

	// Sort the leftover patch keys so the addition order is deterministic.
	leftover := make([]string, 0, len(patch))
	for k := range patch {
		if !handled[k] {
			leftover = append(leftover, k)
		}
	}
	sort.Strings(leftover)
	for _, k := range leftover {
		entry := patch[k]
		if entry == nil || entry.Content == nil {
			// null on a non-existing key is a no-op; new files need content.
			continue
		}
		name := k
		if entry.Filename != nil {
			name = *entry.Filename
		}
		merged = append(merged, db.FileDTO{Filename: name, Content: *entry.Content})
	}

	// Clean filenames + check for collisions in the post-merge list.
	seen := make(map[string]bool, len(merged))
	for i, f := range merged {
		clean := strings.TrimSpace(git.CleanTreePathName(f.Filename))
		if clean == "" {
			return nil, fmt.Errorf("files: filename cannot be empty")
		}
		if seen[clean] {
			return nil, fmt.Errorf("files: duplicate filename after merge: %s", clean)
		}
		seen[clean] = true
		merged[i].Filename = clean
	}
	return merged, nil
}
