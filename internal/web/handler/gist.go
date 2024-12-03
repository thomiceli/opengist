package handler

import (
	"archive/zip"
	"bufio"
	"bytes"
	gojson "encoding/json"
	"errors"
	"fmt"
	"github.com/thomiceli/opengist/internal/web/context"
	"github.com/thomiceli/opengist/internal/web/server"
	"html/template"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/git"
	"github.com/thomiceli/opengist/internal/i18n"
	"github.com/thomiceli/opengist/internal/index"
	"github.com/thomiceli/opengist/internal/render"
	"github.com/thomiceli/opengist/internal/utils"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"gorm.io/gorm"
)

func GistInit(next context.Handler) context.Handler {
	return func(ctx *context.OGContext) error {
		currUser := ctx.User

		userName := ctx.Param("user")
		gistName := ctx.Param("gistname")

		switch filepath.Ext(gistName) {
		case ".js":
			ctx.SetData("gistpage", "js")
			gistName = strings.TrimSuffix(gistName, ".js")
		case ".json":
			ctx.SetData("gistpage", "json")
			gistName = strings.TrimSuffix(gistName, ".json")
		case ".git":
			ctx.SetData("gistpage", "git")
			gistName = strings.TrimSuffix(gistName, ".git")
		}

		gist, err := db.GetGist(userName, gistName)
		if err != nil {
			return ctx.NotFound("Gist not found")
		}

		if gist.Private == db.PrivateVisibility {
			if currUser == nil || currUser.ID != gist.UserID {
				return ctx.NotFound("Gist not found")
			}
		}

		ctx.SetData("gist", gist)

		if config.C.SshGit {
			var sshDomain string

			if config.C.SshExternalDomain != "" {
				sshDomain = config.C.SshExternalDomain
			} else {
				sshDomain = strings.Split(ctx.Request().Host, ":")[0]
			}

			if config.C.SshPort == "22" {
				ctx.SetData("sshCloneUrl", sshDomain+":"+userName+"/"+gistName+".git")
			} else {
				ctx.SetData("sshCloneUrl", "ssh://"+sshDomain+":"+config.C.SshPort+"/"+userName+"/"+gistName+".git")
			}
		}

		baseHttpUrl := ctx.GetData("baseHttpUrl").(string)

		if config.C.HttpGit {
			ctx.SetData("httpCloneUrl", baseHttpUrl+"/"+userName+"/"+gistName+".git")
		}

		ctx.SetData("httpCopyUrl", baseHttpUrl+"/"+userName+"/"+gistName)
		ctx.SetData("currentUrl", template.URL(ctx.Request().URL.Path))
		ctx.SetData("embedScript", fmt.Sprintf(`<script src="%s"></script>`, baseHttpUrl+"/"+userName+"/"+gistName+".js"))

		nbCommits, err := gist.NbCommits()
		if err != nil {
			return ctx.ErrorRes(500, "Error fetching number of commits", err)
		}
		ctx.SetData("nbCommits", nbCommits)

		if currUser != nil {
			hasLiked, err := currUser.HasLiked(gist)
			if err != nil {
				return ctx.ErrorRes(500, "Cannot get user like status", err)
			}
			ctx.SetData("hasLiked", hasLiked)
		}

		if gist.Private > 0 {
			ctx.SetData("NoIndex", true)
		}

		return next(ctx)
	}
}

// GistSoftInit try to load a gist (same as gistInit) but does not return a 404 if the gist is not found
// useful for git clients using HTTP to obfuscate the existence of a private gist
func GistSoftInit(next echo.HandlerFunc) context.Handler {
	return func(ctx *context.OGContext) error {
		userName := ctx.Param("user")
		gistName := ctx.Param("gistname")

		gistName = strings.TrimSuffix(gistName, ".git")

		gist, _ := db.GetGist(userName, gistName)
		ctx.SetData("gist", gist)

		return next(ctx)
	}
}

// GistNewPushSoftInit has the same behavior as gistSoftInit but create a new gist empty instead
func GistNewPushSoftInit(next context.Handler) context.Handler {
	return func(ctx *context.OGContext) error {
		ctx.SetData("gist", new(db.Gist))
		return next(ctx)
	}
}

func AllGists(ctx *context.OGContext) error {
	var err error
	var urlPage string

	fromUserStr := ctx.Param("user")
	userLogged := ctx.User
	pageInt := getPage(ctx)

	sort := "created"
	sortText := ctx.TrH("gist.list.sort-by-created")
	order := "desc"
	orderText := ctx.TrH("gist.list.order-by-desc")

	if ctx.QueryParam("sort") == "updated" {
		sort = "updated"
		sortText = ctx.TrH("gist.list.sort-by-updated")
	}

	if ctx.QueryParam("order") == "asc" {
		order = "asc"
		orderText = ctx.TrH("gist.list.order-by-asc")
	}

	ctx.SetData("sort", sortText)
	ctx.SetData("order", orderText)

	var gists []*db.Gist
	var currentUserId uint
	if userLogged != nil {
		currentUserId = userLogged.ID
	} else {
		currentUserId = 0
	}

	if fromUserStr == "" {
		urlctx := ctx.Request().URL.Path
		if strings.HasSuffix(urlctx, "search") {
			ctx.SetData("htmlTitle", ctx.TrH("gist.list.search-results"))
			ctx.SetData("mode", "search")
			ctx.SetData("searchQuery", ctx.QueryParam("q"))
			ctx.SetData("searchQueryUrl", template.URL("&q="+ctx.QueryParam("q")))
			urlPage = "search"
			gists, err = db.GetAllGistsFromSearch(currentUserId, ctx.QueryParam("q"), pageInt-1, sort, order)
		} else if strings.HasSuffix(urlctx, "all") {
			ctx.SetData("htmlTitle", ctx.TrH("gist.list.all"))
			ctx.SetData("mode", "all")
			urlPage = "all"
			gists, err = db.GetAllGistsForCurrentUser(currentUserId, pageInt-1, sort, order)
		}
	} else {
		liked := false
		forked := false

		liked, err = regexp.MatchString(`/[^/]*/liked`, ctx.Request().URL.Path)
		if err != nil {
			return ctx.ErrorRes(500, "Error matching regexp", err)
		}

		forked, err = regexp.MatchString(`/[^/]*/forked`, ctx.Request().URL.Path)
		if err != nil {
			return ctx.ErrorRes(500, "Error matching regexp", err)
		}

		var fromUser *db.User

		fromUser, err = db.GetUserByUsername(fromUserStr)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ctx.NotFound("User not found")
			}
			return ctx.ErrorRes(500, "Error fetching user", err)
		}
		ctx.SetData("fromUser", fromUser)

		if countFromUser, err := db.CountAllGistsFromUser(fromUser.ID, currentUserId); err != nil {
			return ctx.ErrorRes(500, "Error counting gists", err)
		} else {
			ctx.SetData("countFromUser", countFromUser)
		}

		if countLiked, err := db.CountAllGistsLikedByUser(fromUser.ID, currentUserId); err != nil {
			return ctx.ErrorRes(500, "Error counting liked gists", err)
		} else {
			ctx.SetData("countLiked", countLiked)
		}

		if countForked, err := db.CountAllGistsForkedByUser(fromUser.ID, currentUserId); err != nil {
			return ctx.ErrorRes(500, "Error counting forked gists", err)
		} else {
			ctx.SetData("countForked", countForked)
		}

		if liked {
			urlPage = fromUserStr + "/liked"
			ctx.SetData("htmlTitle", ctx.TrH("gist.list.all-liked-by", fromUserStr))
			ctx.SetData("mode", "liked")
			gists, err = db.GetAllGistsLikedByUser(fromUser.ID, currentUserId, pageInt-1, sort, order)
		} else if forked {
			urlPage = fromUserStr + "/forked"
			ctx.SetData("htmlTitle", ctx.TrH("gist.list.all-forked-by", fromUserStr))
			ctx.SetData("mode", "forked")
			gists, err = db.GetAllGistsForkedByUser(fromUser.ID, currentUserId, pageInt-1, sort, order)
		} else {
			urlPage = fromUserStr
			ctx.SetData("htmlTitle", ctx.TrH("gist.list.all-from", fromUserStr))
			ctx.SetData("mode", "fromUser")
			gists, err = db.GetAllGistsFromUser(fromUser.ID, currentUserId, pageInt-1, sort, order)
		}
	}

	renderedGists := make([]*render.RenderedGist, 0, len(gists))
	for _, gist := range gists {
		rendered, err := render.HighlightGistPreview(gist)
		if err != nil {
			log.Error().Err(err).Msg("Error rendering gist preview for " + gist.Identifier() + " - " + gist.PreviewFilename)
		}
		renderedGists = append(renderedGists, &rendered)
	}

	if err != nil {
		return ctx.ErrorRes(500, "Error fetching gists", err)
	}

	if err = paginate(ctx, renderedGists, pageInt, 10, "gists", fromUserStr, 2, "&sort="+sort+"&order="+order); err != nil {
		return ctx.ErrorRes(404, ctx.Tr("error.page-not-found"), nil)
	}

	ctx.SetData("urlPage", urlPage)
	return ctx.HTML_("all.html")
}

func Search(ctx *context.OGContext) error {
	var err error

	content, meta := ParseSearchQueryStr(ctx.QueryParam("q"))
	pageInt := getPage(ctx)

	var currentUserId uint
	userLogged := ctx.User
	if userLogged != nil {
		currentUserId = userLogged.ID
	} else {
		currentUserId = 0
	}

	var visibleGistsIds []uint
	visibleGistsIds, err = db.GetAllGistsVisibleByUser(currentUserId)
	if err != nil {
		return ctx.ErrorRes(500, "Error fetching gists", err)
	}

	gistsIds, nbHits, langs, err := index.SearchGists(content, index.SearchGistMetadata{
		Username:  meta["user"],
		Title:     meta["title"],
		Filename:  meta["filename"],
		Extension: meta["extension"],
		Language:  meta["language"],
	}, visibleGistsIds, pageInt)
	if err != nil {
		return ctx.ErrorRes(500, "Error searching gists", err)
	}

	gists, err := db.GetAllGistsByIds(gistsIds)
	if err != nil {
		return ctx.ErrorRes(500, "Error fetching gists", err)
	}

	renderedGists := make([]*render.RenderedGist, 0, len(gists))
	for _, gist := range gists {
		rendered, err := render.HighlightGistPreview(gist)
		if err != nil {
			log.Error().Err(err).Msg("Error rendering gist preview for " + gist.Identifier() + " - " + gist.PreviewFilename)
		}
		renderedGists = append(renderedGists, &rendered)
	}

	if pageInt > 1 && len(renderedGists) != 0 {
		ctx.SetData("prevPage", pageInt-1)
	}
	if 10*pageInt < int(nbHits) {
		ctx.SetData("nextPage", pageInt+1)
	}
	ctx.SetData("prevLabel", ctx.TrH("pagination.previous"))
	ctx.SetData("nextLabel", ctx.TrH("pagination.next"))
	ctx.SetData("urlPage", "search")
	ctx.SetData("urlParams", template.URL("&q="+ctx.QueryParam("q")))
	ctx.SetData("htmlTitle", ctx.TrH("gist.list.search-results"))
	ctx.SetData("nbHits", nbHits)
	ctx.SetData("gists", renderedGists)
	ctx.SetData("langs", langs)
	ctx.SetData("searchQuery", ctx.QueryParam("q"))
	return ctx.HTML_("search.html")
}

func GistIndex(ctx *context.OGContext) error {
	if ctx.GetData("gistpage") == "js" {
		return GistJs(ctx)
	} else if ctx.GetData("gistpage") == "json" {
		return GistJson(ctx)
	}

	gist := ctx.GetData("gist").(*db.Gist)
	revision := ctx.Param("revision")

	if revision == "" {
		revision = "HEAD"
	}

	files, err := gist.Files(revision, true)
	if _, ok := err.(*git.RevisionNotFoundError); ok {
		return ctx.NotFound("Revision not found")
	} else if err != nil {
		return ctx.ErrorRes(500, "Error fetching files", err)
	}

	renderedFiles := render.HighlightFiles(files)

	ctx.SetData("page", "code")
	ctx.SetData("commit", revision)
	ctx.SetData("files", renderedFiles)
	ctx.SetData("revision", revision)
	ctx.SetData("htmlTitle", gist.Title)
	return ctx.HTML_("gist.html")
}

func GistJson(ctx *context.OGContext) error {
	gist := ctx.GetData("gist").(*db.Gist)
	files, err := gist.Files("HEAD", true)
	if err != nil {
		return ctx.ErrorRes(500, "Error fetching files", err)
	}

	renderedFiles := render.HighlightFiles(files)
	ctx.SetData("files", renderedFiles)

	htmlbuf := bytes.Buffer{}
	w := bufio.NewWriter(&htmlbuf)
	if err = ctx.Echo().Renderer.Render(w, "gist_embed.html", ctx.DataMap(), ctx); err != nil {
		return err
	}
	_ = w.Flush()

	jsUrl, err := url.JoinPath(ctx.GetData("baseHttpUrl").(string), gist.User.Username, gist.Identifier()+".js")
	if err != nil {
		return ctx.ErrorRes(500, "Error joining js url", err)
	}

	cssUrl, err := url.JoinPath(ctx.GetData("baseHttpUrl").(string), server.ManifestEntries["embed.css"].File)
	if err != nil {
		return ctx.ErrorRes(500, "Error joining css url", err)
	}

	return ctx.JSON(200, map[string]interface{}{
		"owner":       gist.User.Username,
		"id":          gist.Identifier(),
		"uuid":        gist.Uuid,
		"title":       gist.Title,
		"description": gist.Description,
		"created_at":  time.Unix(gist.CreatedAt, 0).Format(time.RFC3339),
		"visibility":  gist.VisibilityStr(),
		"files":       renderedFiles,
		"embed": map[string]string{
			"html":    htmlbuf.String(),
			"css":     cssUrl,
			"js":      jsUrl,
			"js_dark": jsUrl + "?dark",
		},
	})
}

func GistJs(ctx *context.OGContext) error {
	if _, exists := ctx.QueryParams()["dark"]; exists {
		ctx.SetData("dark", "dark")
	}

	gist := ctx.GetData("gist").(*db.Gist)
	files, err := gist.Files("HEAD", true)
	if err != nil {
		return ctx.ErrorRes(500, "Error fetching files", err)
	}

	renderedFiles := render.HighlightFiles(files)
	ctx.SetData("files", renderedFiles)

	htmlbuf := bytes.Buffer{}
	w := bufio.NewWriter(&htmlbuf)
	if err = ctx.Echo().Renderer.Render(w, "gist_embed.html", ctx.DataMap(), ctx); err != nil {
		return err
	}
	_ = w.Flush()

	cssUrl, err := url.JoinPath(ctx.GetData("baseHttpUrl").(string), server.ManifestEntries["embed.css"].File)
	if err != nil {
		return ctx.ErrorRes(500, "Error joining css url", err)
	}

	js, err := escapeJavaScriptContent(htmlbuf.String(), cssUrl)
	if err != nil {
		return ctx.ErrorRes(500, "Error escaping JavaScript content", err)
	}
	ctx.Response().Header().Set("Content-Type", "application/javascript")
	return ctx.PlainText(200, js)
}

func Revisions(ctx *context.OGContext) error {
	gist := ctx.GetData("gist").(*db.Gist)
	userName := gist.User.Username
	gistName := gist.Identifier()

	pageInt := getPage(ctx)

	commits, err := gist.Log((pageInt - 1) * 10)
	if err != nil {
		return ctx.ErrorRes(500, "Error fetching commits log", err)
	}

	if err := paginate(ctx, commits, pageInt, 10, "commits", userName+"/"+gistName+"/revisions", 2); err != nil {
		return ctx.ErrorRes(404, ctx.Tr("error.page-not-found"), nil)
	}

	emailsSet := map[string]struct{}{}
	for _, commit := range commits {
		if commit.AuthorEmail == "" {
			continue
		}
		emailsSet[strings.ToLower(commit.AuthorEmail)] = struct{}{}
	}

	emailsUsers, err := db.GetUsersFromEmails(emailsSet)
	if err != nil {
		return ctx.ErrorRes(500, "Error fetching users emails", err)
	}

	ctx.SetData("page", "revisions")
	ctx.SetData("revision", "HEAD")
	ctx.SetData("emails", emailsUsers)
	ctx.SetData("htmlTitle", ctx.TrH("gist.revision-of", gist.Title))

	return ctx.HTML_("revisions.html")
}

func Create(ctx *context.OGContext) error {
	ctx.SetData("htmlTitle", ctx.TrH("gist.new.create-a-new-gist"))
	return ctx.HTML_("create.html")
}

func ProcessCreate(ctx *context.OGContext) error {
	isCreate := false
	if ctx.Request().URL.Path == "/" {
		isCreate = true
	}

	err := ctx.Request().ParseForm()
	if err != nil {
		return ctx.ErrorRes(400, ctx.Tr("error.bad-request"), err)
	}

	dto := new(db.GistDTO)
	var gist *db.Gist

	if isCreate {
		ctx.SetData("htmlTitle", ctx.TrH("gist.new.create-a-new-gist"))
	} else {
		gist = ctx.GetData("gist").(*db.Gist)
		ctx.SetData("htmlTitle", ctx.TrH("gist.edit.edit-gist", gist.Title))
	}

	if err := ctx.Bind(dto); err != nil {
		return ctx.ErrorRes(400, ctx.Tr("error.cannot-bind-data"), err)
	}

	dto.Files = make([]db.FileDTO, 0)
	fileCounter := 0
	for i := 0; i < len(ctx.Request().PostForm["content"]); i++ {
		name := ctx.Request().PostForm["name"][i]
		content := ctx.Request().PostForm["content"][i]

		if name == "" {
			fileCounter += 1
			name = "gistfile" + strconv.Itoa(fileCounter) + ".txt"
		}

		escapedValue, err := url.QueryUnescape(content)
		if err != nil {
			return ctx.ErrorRes(400, ctx.Tr("error.invalid-character-unescaped"), err)
		}

		dto.Files = append(dto.Files, db.FileDTO{
			Filename: strings.Trim(name, " "),
			Content:  escapedValue,
		})
	}

	err = ctx.Validate(dto)
	if err != nil {
		ctx.AddFlash(utils.ValidationMessages(&err, ctx.GetData("locale").(*i18n.Locale)), "error")
		if isCreate {
			return ctx.HTML_("create.html")
		} else {
			files, err := gist.Files("HEAD", false)
			if err != nil {
				return ctx.ErrorRes(500, "Error fetching files", err)
			}
			ctx.SetData("files", files)
			return ctx.HTML_("edit.html")
		}
	}

	if isCreate {
		gist = dto.ToGist()
	} else {
		gist = dto.ToExistingGist(gist)
	}

	user := ctx.User
	gist.NbFiles = len(dto.Files)

	if isCreate {
		uuidGist, err := uuid.NewRandom()
		if err != nil {
			return ctx.ErrorRes(500, "Error creating an UUID", err)
		}
		gist.Uuid = strings.Replace(uuidGist.String(), "-", "", -1)

		gist.UserID = user.ID
		gist.User = *user
	}

	if gist.Title == "" {
		if ctx.Request().PostForm["name"][0] == "" {
			gist.Title = "gist:" + gist.Uuid
		} else {
			gist.Title = ctx.Request().PostForm["name"][0]
		}
	}

	if len(dto.Files) > 0 {
		split := strings.Split(dto.Files[0].Content, "\n")
		if len(split) > 10 {
			gist.Preview = strings.Join(split[:10], "\n")
		} else {
			gist.Preview = dto.Files[0].Content
		}

		gist.PreviewFilename = dto.Files[0].Filename
	}

	if err = gist.InitRepository(); err != nil {
		return ctx.ErrorRes(500, "Error creating the repository", err)
	}

	if err = gist.AddAndCommitFiles(&dto.Files); err != nil {
		return ctx.ErrorRes(500, "Error adding and committing files", err)
	}

	if isCreate {
		if err = gist.Create(); err != nil {
			return ctx.ErrorRes(500, "Error creating the gist", err)
		}
	} else {
		if err = gist.Update(); err != nil {
			return ctx.ErrorRes(500, "Error updating the gist", err)
		}
	}

	gist.AddInIndex()

	return ctx.RedirectTo("/" + user.Username + "/" + gist.Identifier())
}

func EditVisibility(ctx *context.OGContext) error {
	gist := ctx.GetData("gist").(*db.Gist)

	dto := new(db.VisibilityDTO)
	if err := ctx.Bind(dto); err != nil {
		return ctx.ErrorRes(400, ctx.Tr("error.cannot-bind-data"), err)
	}

	gist.Private = dto.Private
	if err := gist.UpdateNoTimestamps(); err != nil {
		return ctx.ErrorRes(500, "Error updating this gist", err)
	}

	ctx.AddFlash(ctx.Tr("flash.gist.visibility-changed"), "success")
	return ctx.RedirectTo("/" + gist.User.Username + "/" + gist.Identifier())
}

func DeleteGist(ctx *context.OGContext) error {
	gist := ctx.GetData("gist").(*db.Gist)

	if err := gist.Delete(); err != nil {
		return ctx.ErrorRes(500, "Error deleting this gist", err)
	}
	gist.RemoveFromIndex()

	ctx.AddFlash(ctx.Tr("flash.gist.deleted"), "success")
	return ctx.RedirectTo("/")
}

func Like(ctx *context.OGContext) error {
	gist := ctx.GetData("gist").(*db.Gist)
	currentUser := ctx.User

	hasLiked, err := currentUser.HasLiked(gist)
	if err != nil {
		return ctx.ErrorRes(500, "Error checking if user has liked a gist", err)
	}

	if hasLiked {
		err = gist.RemoveUserLike(ctx.User)
	} else {
		err = gist.AppendUserLike(ctx.User)
	}

	if err != nil {
		return ctx.ErrorRes(500, "Error liking/dislking this gist", err)
	}

	redirectTo := "/" + gist.User.Username + "/" + gist.Identifier()
	if r := ctx.QueryParam("redirecturl"); r != "" {
		redirectTo = r
	}
	return ctx.RedirectTo(redirectTo)
}

func Fork(ctx *context.OGContext) error {
	gist := ctx.GetData("gist").(*db.Gist)
	currentUser := ctx.User

	alreadyForked, err := gist.GetForkParent(currentUser)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return ctx.ErrorRes(500, "Error checking if gist is already forked", err)
	}

	if gist.User.ID == currentUser.ID {
		ctx.AddFlash(ctx.Tr("flash.gist.fork-own-gist"), "error")
		return ctx.RedirectTo("/" + gist.User.Username + "/" + gist.Identifier())
	}

	if alreadyForked.ID != 0 {
		return ctx.RedirectTo("/" + alreadyForked.User.Username + "/" + alreadyForked.Identifier())
	}

	uuidGist, err := uuid.NewRandom()
	if err != nil {
		return ctx.ErrorRes(500, "Error creating an UUID", err)
	}

	newGist := &db.Gist{
		Uuid:            strings.Replace(uuidGist.String(), "-", "", -1),
		Title:           gist.Title,
		Preview:         gist.Preview,
		PreviewFilename: gist.PreviewFilename,
		Description:     gist.Description,
		Private:         gist.Private,
		UserID:          currentUser.ID,
		ForkedID:        gist.ID,
		NbFiles:         gist.NbFiles,
	}

	if err = newGist.CreateForked(); err != nil {
		return ctx.ErrorRes(500, "Error forking the gist in database", err)
	}

	if err = gist.ForkClone(currentUser.Username, newGist.Uuid); err != nil {
		return ctx.ErrorRes(500, "Error cloning the repository while forking", err)
	}
	if err = gist.IncrementForkCount(); err != nil {
		return ctx.ErrorRes(500, "Error incrementing the fork count", err)
	}

	ctx.AddFlash(ctx.Tr("flash.gist.forked"), "success")

	return ctx.RedirectTo("/" + currentUser.Username + "/" + newGist.Identifier())
}

func RawFile(ctx *context.OGContext) error {
	gist := ctx.GetData("gist").(*db.Gist)
	file, err := gist.File(ctx.Param("revision"), ctx.Param("file"), false)
	if err != nil {
		return ctx.ErrorRes(500, "Error getting file content", err)
	}

	if file == nil {
		return ctx.NotFound("File not found")
	}

	return ctx.PlainText(200, file.Content)
}

func DownloadFile(ctx *context.OGContext) error {
	gist := ctx.GetData("gist").(*db.Gist)
	file, err := gist.File(ctx.Param("revision"), ctx.Param("file"), false)
	if err != nil {
		return ctx.ErrorRes(500, "Error getting file content", err)
	}

	if file == nil {
		return ctx.NotFound("File not found")
	}

	ctx.Response().Header().Set("Content-Type", "text/plain")
	ctx.Response().Header().Set("Content-Disposition", "attachment; filename="+file.Filename)
	ctx.Response().Header().Set("Content-Length", strconv.Itoa(len(file.Content)))
	_, err = ctx.Response().Write([]byte(file.Content))
	if err != nil {
		return ctx.ErrorRes(500, "Error downloading the file", err)
	}

	return nil
}

func Edit(ctx *context.OGContext) error {
	gist := ctx.GetData("gist").(*db.Gist)

	files, err := gist.Files("HEAD", false)
	if err != nil {
		return ctx.ErrorRes(500, "Error fetching files from repository", err)
	}

	ctx.SetData("files", files)
	ctx.SetData("htmlTitle", ctx.TrH("gist.edit.edit-gist", gist.Title))

	return ctx.HTML_("edit.html")
}

func DownloadZip(ctx *context.OGContext) error {
	gist := ctx.GetData("gist").(*db.Gist)
	revision := ctx.Param("revision")

	files, err := gist.Files(revision, false)
	if err != nil {
		return ctx.ErrorRes(500, "Error fetching files from repository", err)
	}
	if len(files) == 0 {
		return ctx.NotFound("No files found in this revision")
	}

	zipFile := new(bytes.Buffer)

	zipWriter := zip.NewWriter(zipFile)

	for _, file := range files {
		fh := &zip.FileHeader{
			Name:   file.Filename,
			Method: zip.Deflate,
		}
		f, err := zipWriter.CreateHeader(fh)
		if err != nil {
			return ctx.ErrorRes(500, "Error adding a file the to the zip archive", err)
		}
		_, err = f.Write([]byte(file.Content))
		if err != nil {
			return ctx.ErrorRes(500, "Error adding file content the to the zip archive", err)
		}
	}
	err = zipWriter.Close()
	if err != nil {
		return ctx.ErrorRes(500, "Error closing the zip archive", err)
	}

	ctx.Response().Header().Set("Content-Type", "application/zip")
	ctx.Response().Header().Set("Content-Disposition", "attachment; filename="+gist.Identifier()+".zip")
	ctx.Response().Header().Set("Content-Length", strconv.Itoa(len(zipFile.Bytes())))
	_, err = ctx.Response().Write(zipFile.Bytes())
	if err != nil {
		return ctx.ErrorRes(500, "Error writing the zip archive", err)
	}
	return nil
}

func Likes(ctx *context.OGContext) error {
	gist := ctx.GetData("gist").(*db.Gist)

	pageInt := getPage(ctx)

	likers, err := gist.GetUsersLikes(pageInt - 1)
	if err != nil {
		return ctx.ErrorRes(500, "Error getting users who liked this gist", err)
	}

	if err = paginate(ctx, likers, pageInt, 30, "likers", gist.User.Username+"/"+gist.Identifier()+"/likes", 1); err != nil {
		return ctx.ErrorRes(404, ctx.Tr("error.page-not-found"), nil)
	}

	ctx.SetData("htmlTitle", ctx.TrH("gist.likes.for", gist.Title))
	ctx.SetData("revision", "HEAD")
	return ctx.HTML_("likes.html")
}

func Forks(ctx *context.OGContext) error {
	gist := ctx.GetData("gist").(*db.Gist)
	pageInt := getPage(ctx)

	currentUser := ctx.User
	var fromUserID uint = 0
	if currentUser != nil {
		fromUserID = currentUser.ID
	}

	forks, err := gist.GetForks(fromUserID, pageInt-1)
	if err != nil {
		return ctx.ErrorRes(500, "Error getting users who liked this gist", err)
	}

	if err = paginate(ctx, forks, pageInt, 30, "forks", gist.User.Username+"/"+gist.Identifier()+"/forks", 2); err != nil {
		return ctx.ErrorRes(404, ctx.Tr("error.page-not-found"), nil)
	}

	ctx.SetData("htmlTitle", ctx.TrH("gist.forks.for", gist.Title))
	ctx.SetData("revision", "HEAD")
	return ctx.HTML_("forks.html")
}

func Checkbox(ctx *context.OGContext) error {
	filename := ctx.FormValue("file")
	checkboxNb := ctx.FormValue("checkbox")

	i, err := strconv.Atoi(checkboxNb)
	if err != nil {
		return ctx.ErrorRes(400, ctx.Tr("error.invalid-number"), nil)
	}

	gist := ctx.GetData("gist").(*db.Gist)
	file, err := gist.File("HEAD", filename, false)
	if err != nil {
		return ctx.ErrorRes(500, "Error getting file content", err)
	} else if file == nil {
		return ctx.NotFound("File not found")
	}

	markdown, err := render.Checkbox(file.Content, i)
	if err != nil {
		return ctx.ErrorRes(500, "Error checking checkbox", err)
	}

	if err = gist.AddAndCommitFile(&db.FileDTO{
		Filename: filename,
		Content:  markdown,
	}); err != nil {
		return ctx.ErrorRes(500, "Error adding and committing files", err)
	}

	if err = gist.UpdatePreviewAndCount(true); err != nil {
		return ctx.ErrorRes(500, "Error updating the gist", err)
	}

	return ctx.PlainText(200, "ok")
}

func Preview(ctx *context.OGContext) error {
	content := ctx.FormValue("content")

	previewStr, err := render.MarkdownString(content)
	if err != nil {
		return ctx.ErrorRes(500, "Error rendering markdown", err)
	}

	return ctx.PlainText(200, previewStr)
}

func escapeJavaScriptContent(htmlContent, cssUrl string) (string, error) {
	jsonContent, err := gojson.Marshal(htmlContent)
	if err != nil {
		return "", fmt.Errorf("failed to encode content: %w", err)
	}

	jsonCssUrl, err := gojson.Marshal(cssUrl)
	if err != nil {
		return "", fmt.Errorf("failed to encode CSS URL: %w", err)
	}

	js := fmt.Sprintf(`
        document.write('<link rel="stylesheet" href=%s>');
        document.write(%s);
    `,
		string(jsonCssUrl),
		string(jsonContent),
	)

	return js, nil
}
