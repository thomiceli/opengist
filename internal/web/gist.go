package web

import (
	"archive/zip"
	"bytes"
	"errors"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/models"
	"gorm.io/gorm"
	"html/template"
	"net/url"
	"strconv"
	"strings"
)

func gistInit(next echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		userName := ctx.Param("user")
		gistName := ctx.Param("gistname")

		gistName = strings.TrimSuffix(gistName, ".git")

		gist, err := models.GetGist(userName, gistName)
		if err != nil {
			return notFound("Gist not found")
		}
		setData(ctx, "gist", gist)

		if config.C.SshGit {
			var sshDomain string

			if config.C.SshExternalDomain != "" {
				sshDomain = config.C.SshExternalDomain
			} else {
				sshDomain = strings.Split(ctx.Request().Host, ":")[0]
			}

			if config.C.SshPort == "22" {
				setData(ctx, "sshCloneUrl", sshDomain+":"+userName+"/"+gistName+".git")
			} else {
				setData(ctx, "sshCloneUrl", "ssh://"+sshDomain+":"+config.C.SshPort+"/"+userName+"/"+gistName+".git")
			}
		}

		httpProtocol := "http"
		if ctx.Request().TLS != nil || ctx.Request().Header.Get("X-Forwarded-Proto") == "https" {
			httpProtocol = "https"
		}
		setData(ctx, "httpProtocol", strings.ToUpper(httpProtocol))

		var baseHttpUrl string
		// if a custom external url is set, use it
		if config.C.ExternalUrl != "" {
			baseHttpUrl = config.C.ExternalUrl
		} else {
			baseHttpUrl = httpProtocol + "://" + ctx.Request().Host
		}

		if config.C.HttpGit {
			setData(ctx, "httpCloneUrl", baseHttpUrl+"/"+userName+"/"+gistName+".git")
		}

		setData(ctx, "httpCopyUrl", baseHttpUrl+"/"+userName+"/"+gistName)
		setData(ctx, "currentUrl", template.URL(ctx.Request().URL.Path))

		nbCommits, err := gist.NbCommits()
		if err != nil {
			return errorRes(500, "Error fetching number of commits", err)
		}
		setData(ctx, "nbCommits", nbCommits)

		if currUser := getUserLogged(ctx); currUser != nil {
			hasLiked, err := currUser.HasLiked(gist)
			if err != nil {
				return errorRes(500, "Cannot get user like status", err)
			}
			setData(ctx, "hasLiked", hasLiked)
		}

		return next(ctx)
	}
}

func allGists(ctx echo.Context) error {
	var err error
	fromUserStr := ctx.Param("user")

	userLogged := getUserLogged(ctx)

	pageInt := getPage(ctx)

	sort := "created"
	order := "desc"
	orderText := "Recently"

	if ctx.QueryParam("sort") == "updated" {
		sort = "updated"
	}

	if ctx.QueryParam("order") == "asc" {
		order = "asc"
		orderText = "Least recently"
	}

	setData(ctx, "sort", sort)
	setData(ctx, "order", orderText)

	var gists []*models.Gist
	var currentUserId uint
	if userLogged != nil {
		currentUserId = userLogged.ID
	} else {
		currentUserId = 0
	}
	if fromUserStr == "" {
		setData(ctx, "htmlTitle", "All gists")
		fromUserStr = "all"
		gists, err = models.GetAllGistsForCurrentUser(currentUserId, pageInt-1, sort, order)
	} else {
		setData(ctx, "htmlTitle", "All gists from "+fromUserStr)

		var fromUser *models.User
		fromUser, err = models.GetUserByUsername(fromUserStr)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return notFound("User not found")
			}
			return errorRes(500, "Error fetching user", err)
		}
		setData(ctx, "fromUser", fromUser)

		gists, err = models.GetAllGistsFromUser(fromUserStr, currentUserId, pageInt-1, sort, order)
	}

	if err != nil {
		return errorRes(500, "Error fetching gists", err)
	}

	if err = paginate(ctx, gists, pageInt, 10, "gists", fromUserStr, 2, "&sort="+sort+"&order="+order); err != nil {
		return errorRes(404, "Page not found", nil)
	}

	return html(ctx, "all.html")
}

func gistIndex(ctx echo.Context) error {
	gist := getData(ctx, "gist").(*models.Gist)
	revision := ctx.Param("revision")

	if revision == "" {
		revision = "HEAD"
	}

	files, err := gist.Files(revision)
	if err != nil {
		return errorRes(500, "Error fetching files", err)
	}

	if len(files) == 0 {
		return notFound("Revision not found")
	}

	setData(ctx, "page", "code")
	setData(ctx, "commit", revision)
	setData(ctx, "files", files)
	setData(ctx, "revision", revision)
	setData(ctx, "htmlTitle", gist.Title)
	return html(ctx, "gist.html")
}

func revisions(ctx echo.Context) error {
	gist := getData(ctx, "gist").(*models.Gist)
	userName := gist.User.Username
	gistName := gist.Uuid

	pageInt := getPage(ctx)

	commits, err := gist.Log((pageInt - 1) * 10)
	if err != nil {
		return errorRes(500, "Error fetching commits log", err)
	}

	if err := paginate(ctx, commits, pageInt, 10, "commits", userName+"/"+gistName+"/revisions", 2); err != nil {
		return errorRes(404, "Page not found", nil)
	}

	setData(ctx, "page", "revisions")
	setData(ctx, "revision", "HEAD")
	setData(ctx, "htmlTitle", "Revision of "+gist.Title)

	return html(ctx, "revisions.html")
}

func create(ctx echo.Context) error {
	setData(ctx, "htmlTitle", "Create a new gist")
	return html(ctx, "create.html")
}

func processCreate(ctx echo.Context) error {
	isCreate := false
	if ctx.Request().URL.Path == "/" {
		isCreate = true
	}

	err := ctx.Request().ParseForm()
	if err != nil {
		return errorRes(400, "Bad request", err)
	}

	dto := new(models.GistDTO)
	var gist *models.Gist

	if isCreate {
		setData(ctx, "htmlTitle", "Create a new gist")
	} else {
		gist = getData(ctx, "gist").(*models.Gist)
		setData(ctx, "htmlTitle", "Edit "+gist.Title)
	}

	if err := ctx.Bind(dto); err != nil {
		return errorRes(400, "Cannot bind data", err)
	}

	dto.Files = make([]models.FileDTO, 0)
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
			return errorRes(400, "Invalid character unescaped", err)
		}

		dto.Files = append(dto.Files, models.FileDTO{
			Filename: strings.Trim(name, " "),
			Content:  escapedValue,
		})
	}

	err = ctx.Validate(dto)
	if err != nil {
		addFlash(ctx, validationMessages(&err), "error")
		if isCreate {
			return html(ctx, "create.html")
		} else {
			files, err := gist.Files("HEAD")
			if err != nil {
				return errorRes(500, "Error fetching files", err)
			}
			setData(ctx, "files", files)
			return html(ctx, "edit.html")
		}
	}

	if isCreate {
		gist = dto.ToGist()
	} else {
		gist = dto.ToExistingGist(gist)
	}

	user := getUserLogged(ctx)
	gist.NbFiles = len(dto.Files)

	if isCreate {
		uuidGist, err := uuid.NewRandom()
		if err != nil {
			return errorRes(500, "Error creating an UUID", err)
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
		return errorRes(500, "Error creating the repository", err)
	}

	if err = gist.AddAndCommitFiles(&dto.Files); err != nil {
		return errorRes(500, "Error adding and commiting files", err)
	}

	if isCreate {
		if err = gist.Create(); err != nil {
			return errorRes(500, "Error creating the gist", err)
		}
	} else {
		if err = gist.Update(); err != nil {
			return errorRes(500, "Error updating the gist", err)
		}
	}

	return redirect(ctx, "/"+user.Username+"/"+gist.Uuid)
}

func toggleVisibility(ctx echo.Context) error {
	var gist = getData(ctx, "gist").(*models.Gist)

	gist.Private = !gist.Private
	if err := gist.Update(); err != nil {
		return errorRes(500, "Error updating this gist", err)
	}

	addFlash(ctx, "Gist visibility has been changed", "success")
	return redirect(ctx, "/"+gist.User.Username+"/"+gist.Uuid)
}

func deleteGist(ctx echo.Context) error {
	var gist = getData(ctx, "gist").(*models.Gist)

	err := gist.DeleteRepository()
	if err != nil {
		return errorRes(500, "Error deleting the repository", err)
	}

	if err := gist.Delete(); err != nil {
		return errorRes(500, "Error deleting this gist", err)
	}

	addFlash(ctx, "Gist has been deleted", "success")
	return redirect(ctx, "/")
}

func like(ctx echo.Context) error {
	var gist = getData(ctx, "gist").(*models.Gist)
	currentUser := getUserLogged(ctx)

	hasLiked, err := currentUser.HasLiked(gist)
	if err != nil {
		return errorRes(500, "Error checking if user has liked a gist", err)
	}

	if hasLiked {
		err = gist.RemoveUserLike(getUserLogged(ctx))
	} else {
		err = gist.AppendUserLike(getUserLogged(ctx))
	}

	if err != nil {
		return errorRes(500, "Error liking/dislking this gist", err)
	}

	redirectTo := "/" + gist.User.Username + "/" + gist.Uuid
	if r := ctx.QueryParam("redirecturl"); r != "" {
		redirectTo = r
	}
	return redirect(ctx, redirectTo)
}

func fork(ctx echo.Context) error {
	var gist = getData(ctx, "gist").(*models.Gist)
	currentUser := getUserLogged(ctx)

	alreadyForked, err := gist.GetForkParent(currentUser)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return errorRes(500, "Error checking if gist is already forked", err)
	}

	if gist.User.ID == currentUser.ID {
		addFlash(ctx, "Unable to fork own gists", "error")
		return redirect(ctx, "/"+gist.User.Username+"/"+gist.Uuid)
	}

	if alreadyForked.ID != 0 {
		return redirect(ctx, "/"+alreadyForked.User.Username+"/"+alreadyForked.Uuid)
	}

	uuidGist, err := uuid.NewRandom()
	if err != nil {
		return errorRes(500, "Error creating an UUID", err)
	}

	newGist := &models.Gist{
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
		return errorRes(500, "Error forking the gist in database", err)
	}

	if err = gist.ForkClone(currentUser.Username, newGist.Uuid); err != nil {
		return errorRes(500, "Error cloning the repository while forking", err)
	}
	if err = gist.IncrementForkCount(); err != nil {
		return errorRes(500, "Error incrementing the fork count", err)
	}

	addFlash(ctx, "Gist has been forked", "success")

	return redirect(ctx, "/"+currentUser.Username+"/"+newGist.Uuid)
}

func rawFile(ctx echo.Context) error {
	gist := getData(ctx, "gist").(*models.Gist)
	file, err := gist.File(ctx.Param("revision"), ctx.Param("file"), false)

	if err != nil {
		return errorRes(500, "Error getting file content", err)
	}

	if file == nil {
		return notFound("File not found")
	}

	return plainText(ctx, 200, file.Content)
}

func edit(ctx echo.Context) error {
	var gist = getData(ctx, "gist").(*models.Gist)

	files, err := gist.Files("HEAD")
	if err != nil {
		return errorRes(500, "Error fetching files from repository", err)
	}

	setData(ctx, "files", files)
	setData(ctx, "htmlTitle", "Edit "+gist.Title)

	return html(ctx, "edit.html")
}

func downloadZip(ctx echo.Context) error {
	var gist = getData(ctx, "gist").(*models.Gist)
	var revision = ctx.Param("revision")

	files, err := gist.Files(revision)
	if err != nil {
		return errorRes(500, "Error fetching files from repository", err)
	}
	if len(files) == 0 {
		return notFound("No files found in this revision")
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
			return errorRes(500, "Error adding a file the to the zip archive", err)
		}
		_, err = f.Write([]byte(file.Content))
		if err != nil {
			return errorRes(500, "Error adding file content the to the zip archive", err)
		}
	}
	err = zipWriter.Close()
	if err != nil {
		return errorRes(500, "Error closing the zip archive", err)
	}

	ctx.Response().Header().Set("Content-Type", "application/zip")
	ctx.Response().Header().Set("Content-Disposition", "attachment; filename="+gist.Uuid+".zip")
	ctx.Response().Header().Set("Content-Length", strconv.Itoa(len(zipFile.Bytes())))
	_, err = ctx.Response().Write(zipFile.Bytes())
	if err != nil {
		return errorRes(500, "Error writing the zip archive", err)
	}
	return nil
}

func likes(ctx echo.Context) error {
	var gist = getData(ctx, "gist").(*models.Gist)

	pageInt := getPage(ctx)

	likers, err := gist.GetUsersLikes(pageInt - 1)
	if err != nil {
		return errorRes(500, "Error getting users who liked this gist", err)
	}

	if err = paginate(ctx, likers, pageInt, 30, "likers", gist.User.Username+"/"+gist.Uuid+"/likes", 1); err != nil {
		return errorRes(404, "Page not found", nil)
	}

	setData(ctx, "htmlTitle", "Likes for "+gist.Title)
	setData(ctx, "revision", "HEAD")
	return html(ctx, "likes.html")
}

func forks(ctx echo.Context) error {
	var gist = getData(ctx, "gist").(*models.Gist)
	pageInt := getPage(ctx)

	currentUser := getUserLogged(ctx)
	var fromUserID uint = 0
	if currentUser != nil {
		fromUserID = currentUser.ID
	}

	forks, err := gist.GetForks(fromUserID, pageInt-1)
	if err != nil {
		return errorRes(500, "Error getting users who liked this gist", err)
	}

	if err = paginate(ctx, forks, pageInt, 30, "forks", gist.User.Username+"/"+gist.Uuid+"/forks", 2); err != nil {
		return errorRes(404, "Page not found", nil)
	}

	setData(ctx, "htmlTitle", "Forks for "+gist.Title)
	setData(ctx, "revision", "HEAD")
	return html(ctx, "forks.html")
}
