package v1

import (
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/git"
	"github.com/thomiceli/opengist/internal/web/context"
)

const (
	defaultPerPage = 10
	maxPerPage     = 100
)

func parsePagination(ctx *context.Context) (page, perPage int) {
	page, _ = strconv.Atoi(ctx.QueryParam("page"))
	if page < 1 {
		page = 1
	}
	perPage, _ = strconv.Atoi(ctx.QueryParam("per_page"))
	if perPage < 1 {
		perPage = defaultPerPage
	}
	if perPage > maxPerPage {
		perPage = maxPerPage
	}
	return
}

func summarizeGist(g *db.Gist) GistSummary {
	files, _, _ := g.Files("HEAD", true)
	fs := make([]FileSummary, 0, len(files))
	for _, f := range files {
		fs = append(fs, FileSummary{
			Filename: f.Filename,
			Size:     int(f.Size),
			// Language is omitted in v1 (no per-file language detection in db layer)
		})
	}
	return GistSummary{
		UUID:        g.Uuid,
		Title:       g.Title,
		Description: g.Description,
		Visibility:  g.Private.String(),
		HTMLURL:     "/" + g.User.Username + "/" + g.Identifier(),
		CreatedAt:   time.Unix(g.CreatedAt, 0).UTC(),
		UpdatedAt:   time.Unix(g.UpdatedAt, 0).UTC(),
		Owner:       GistOwner{ID: g.User.ID, Username: g.User.Username},
		Files:       fs,
	}
}

func detailGist(g *db.Gist) (GistDetail, error) {
	files, _, err := g.Files("HEAD", true)
	if err != nil {
		return GistDetail{}, err
	}
	fs := make([]FileDetail, 0, len(files))
	for _, f := range files {
		fd := FileDetail{
			Filename:  f.Filename,
			Size:      int(f.Size),
			Truncated: f.Truncated,
		}
		if f.MimeType.CanBeEdited() {
			fd.Content = f.Content
		} else {
			fd.Binary = true
		}
		fs = append(fs, fd)
	}
	return GistDetail{
		UUID:        g.Uuid,
		Title:       g.Title,
		Description: g.Description,
		Visibility:  g.Private.String(),
		HTMLURL:     "/" + g.User.Username + "/" + g.Identifier(),
		CreatedAt:   time.Unix(g.CreatedAt, 0).UTC(),
		UpdatedAt:   time.Unix(g.UpdatedAt, 0).UTC(),
		Owner:       GistOwner{ID: g.User.ID, Username: g.User.Username},
		Files:       fs,
	}, nil
}

// lookupGistByUUID fetches a gist by UUID and enforces visibility.
// Returns 404 (via ErrorBody.Code) if the gist doesn't exist OR is private and the
// caller is not its owner.
func lookupGistByUUID(ctx *context.Context, uuid string) (*db.Gist, *ErrorBody) {
	g, err := db.GetGistByUUID(uuid)
	if err != nil {
		return nil, &ErrorBody{Code: "not_found", Message: "gist not found"}
	}
	if g.Private == db.PrivateVisibility {
		if ctx.User == nil || ctx.User.ID != g.UserID {
			return nil, &ErrorBody{Code: "not_found", Message: "gist not found"}
		}
	}
	return g, nil
}

// GetGist handles GET /api/v1/gists/:uuid
func GetGist(ctx *context.Context) error {
	g, errBody := lookupGistByUUID(ctx, ctx.Param("uuid"))
	if errBody != nil {
		return WriteJSONError(ctx, 404, errBody.Code, errBody.Message)
	}
	resp, err := detailGist(g)
	if err != nil {
		return WriteJSONError(ctx, 500, "internal_error", "failed to serialize gist")
	}
	return ctx.JSON(200, resp)
}

// ListGists handles GET /api/v1/gists?page=&per_page=&visibility=mine|public
func ListGists(ctx *context.Context) error {
	page, perPage := parsePagination(ctx)
	visibility := ctx.QueryParam("visibility")
	if visibility == "" {
		visibility = "mine"
	}

	// db.GetAllGistsFromUser uses offset as page index (0-based) and internally
	// applies Offset(offset*10) with a fixed Limit(11).
	pageIdx := page - 1
	user := ctx.User

	var gists []*db.Gist
	var total int64
	var err error

	switch visibility {
	case "mine":
		gists, total, err = db.GetAllGistsFromUser(user.ID, user.ID, "", "", "", nil, pageIdx, "created", "desc")
	case "public":
		gists, total, err = db.GetAllPublicGists(pageIdx)
	default:
		return WriteJSONError(ctx, 400, "validation_failed", "unknown visibility (allowed: mine, public)")
	}
	if err != nil {
		return WriteJSONError(ctx, 500, "internal_error", "failed to list gists")
	}

	// db.GetAllGistsFromUser doesn't accept a limit; trim manually.
	if len(gists) > perPage {
		gists = gists[:perPage]
	}

	data := make([]GistSummary, 0, len(gists))
	for _, g := range gists {
		data = append(data, summarizeGist(g))
	}

	return ctx.JSON(200, PaginatedGists{
		Data: data,
		Pagination: Pagination{
			Page:    page,
			PerPage: perPage,
			Total:   total,
		},
	})
}

// UpdateGist handles PATCH /api/v1/gists/:uuid
func UpdateGist(ctx *context.Context) error {
	g, errBody := lookupGistByUUID(ctx, ctx.Param("uuid"))
	if errBody != nil {
		return WriteJSONError(ctx, 404, errBody.Code, errBody.Message)
	}
	// non-owners get 404 (don't reveal existence)
	if g.UserID != ctx.User.ID {
		return WriteJSONError(ctx, 404, "not_found", "gist not found")
	}

	var req UpdateGistRequest
	if err := ctx.Bind(&req); err != nil {
		return WriteJSONError(ctx, 400, "validation_failed", "invalid JSON body")
	}

	if req.Title != nil {
		g.Title = *req.Title
	}
	if req.Description != nil {
		g.Description = *req.Description
	}
	if req.Visibility != nil {
		g.Private = db.ParseVisibility[string](*req.Visibility)
	}

	if req.Files != nil {
		if len(*req.Files) == 0 {
			return WriteJSONError(ctx, 400, "validation_failed", "files: at least one file required when provided")
		}
		fileDTOs := make([]db.FileDTO, 0, len(*req.Files))
		for _, f := range *req.Files {
			name := git.CleanTreePathName(f.Filename)
			if name == "" {
				return WriteJSONError(ctx, 400, "validation_failed", "files: filename cannot be empty")
			}
			fileDTOs = append(fileDTOs, db.FileDTO{Filename: name, Content: f.Content})
		}
		g.NbFiles = len(fileDTOs)
		if err := g.AddAndCommitFiles(&fileDTOs); err != nil {
			return WriteJSONError(ctx, 500, "internal_error", "failed to commit files")
		}
	}

	if err := g.Update(); err != nil {
		return WriteJSONError(ctx, 500, "internal_error", "failed to update gist")
	}
	g.UpdateLanguages()
	_ = g.UpdatePreviewAndCount(true)

	resp, err := detailGist(g)
	if err != nil {
		return WriteJSONError(ctx, 500, "internal_error", "failed to serialize gist")
	}
	return ctx.JSON(200, resp)
}

// DeleteGist handles DELETE /api/v1/gists/:uuid
func DeleteGist(ctx *context.Context) error {
	g, errBody := lookupGistByUUID(ctx, ctx.Param("uuid"))
	if errBody != nil {
		return WriteJSONError(ctx, 404, errBody.Code, errBody.Message)
	}
	if g.UserID != ctx.User.ID {
		return WriteJSONError(ctx, 404, "not_found", "gist not found")
	}
	if err := g.DeleteRepository(); err != nil {
		return WriteJSONError(ctx, 500, "internal_error", "failed to delete repository")
	}
	if err := g.Delete(); err != nil {
		return WriteJSONError(ctx, 500, "internal_error", "failed to delete gist")
	}
	return ctx.NoContent(204)
}

// RawFile handles GET /api/v1/gists/:uuid/files/:filename/raw
func RawFile(ctx *context.Context) error {
	g, errBody := lookupGistByUUID(ctx, ctx.Param("uuid"))
	if errBody != nil {
		return WriteJSONError(ctx, 404, errBody.Code, errBody.Message)
	}
	filename := ctx.Param("filename")

	content, _, err := git.GetFileContent(g.User.Username, g.Uuid, "HEAD", filename, false)
	if err != nil || content == "" {
		return WriteJSONError(ctx, 404, "not_found", "file not found")
	}

	ctx.Response().Header().Set("Content-Type", "text/plain; charset=utf-8")
	dispo := mime.FormatMediaType("inline", map[string]string{"filename": filename})
	if dispo == "" {
		dispo = `inline`
	}
	ctx.Response().Header().Set("Content-Disposition", dispo)
	return ctx.String(http.StatusOK, content)
}

// CreateGist handles POST /api/v1/gists
func CreateGist(ctx *context.Context) error {
	var req CreateGistRequest
	if err := ctx.Bind(&req); err != nil {
		return WriteJSONError(ctx, 400, "validation_failed", "invalid JSON body")
	}
	if len(req.Files) == 0 {
		return WriteJSONError(ctx, 400, "validation_failed", "files: at least one file required")
	}

	visibility := db.ParseVisibility[string](req.Visibility)

	fileDTOs := make([]db.FileDTO, 0, len(req.Files))
	for _, f := range req.Files {
		name := git.CleanTreePathName(f.Filename)
		if name == "" {
			return WriteJSONError(ctx, 400, "validation_failed", "files: filename cannot be empty")
		}
		fileDTOs = append(fileDTOs, db.FileDTO{Filename: name, Content: f.Content})
	}

	user := ctx.User
	gist := &db.Gist{
		Title:       req.Title,
		Description: req.Description,
		Private:     visibility,
		UserID:      user.ID,
		User:        *user,
		NbFiles:     len(fileDTOs),
	}

	id, err := uuid.NewRandom()
	if err != nil {
		return WriteJSONError(ctx, 500, "internal_error", "uuid generation failed")
	}
	gist.Uuid = strings.ReplaceAll(id.String(), "-", "")
	if gist.Title == "" {
		gist.Title = fileDTOs[0].Filename
	}

	if err := gist.InitRepository(); err != nil {
		return WriteJSONError(ctx, 500, "internal_error", "failed to init repo")
	}
	if err := gist.AddAndCommitFiles(&fileDTOs); err != nil {
		_ = gist.DeleteRepository()
		return WriteJSONError(ctx, 500, "internal_error", "failed to commit files")
	}
	if err := gist.Create(); err != nil {
		_ = gist.DeleteRepository()
		return WriteJSONError(ctx, 500, "internal_error", "failed to create gist")
	}
	gist.AddInIndex()
	gist.UpdateLanguages()
	_ = gist.UpdatePreviewAndCount(true)

	// reload to fetch timestamps
	saved, err := db.GetGistByID(strconv.FormatUint(uint64(gist.ID), 10))
	if err != nil {
		return WriteJSONError(ctx, 500, "internal_error", "failed to reload gist")
	}
	resp, err := detailGist(saved)
	if err != nil {
		return WriteJSONError(ctx, 500, "internal_error", "failed to serialize gist")
	}
	return ctx.JSON(201, resp)
}
