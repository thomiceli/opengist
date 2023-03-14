package web

import (
	"archive/zip"
	"bytes"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"html/template"
	"net/url"
	"opengist/internal/config"
	"opengist/internal/git"
	"opengist/internal/models"
	"strconv"
	"strings"
)

func gistInit(next echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		userName := ctx.Param("user")
		gistName := ctx.Param("gistname")

		if strings.HasSuffix(gistName, ".git") {
			gistName = strings.TrimSuffix(gistName, ".git")
		}

		gist, err := models.GetGist(userName, gistName)
		if err != nil {
			return notFound("Gist not found")
		}
		setData(ctx, "gist", gist)

		if config.C.SSH.Port == "22" {
			setData(ctx, "ssh_clone_url", config.C.SSH.Domain+":"+userName+"/"+gistName+".git")
		} else {
			setData(ctx, "ssh_clone_url", "ssh://"+config.C.SSH.Domain+":"+config.C.SSH.Port+"/"+userName+"/"+gistName+".git")
		}

		setData(ctx, "httpCloneUrl", "http://"+ctx.Request().Host+"/"+userName+"/"+gistName+".git")
		setData(ctx, "httpCopyUrl", "http://"+ctx.Request().Host+"/"+userName+"/"+gistName)

		setData(ctx, "currentUrl", template.URL(ctx.Request().URL.Path))

		nbCommits, err := git.GetNumberOfCommitsOfRepository(userName, gistName)
		if err != nil {
			return errorRes(500, "Error fetching number of commits", err)
		}
		setData(ctx, "nbCommits", nbCommits)

		if currUser := getUserLogged(ctx); currUser != nil {
			hasLiked, err := models.UserHasLikedGist(currUser, gist)
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
	fromUser := ctx.Param("user")
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
	if fromUser == "" {
		setData(ctx, "htmlTitle", "All gists")
		fromUser = "all"
		gists, err = models.GetAllGistsForCurrentUser(currentUserId, pageInt-1, sort, order)
	} else {
		setData(ctx, "htmlTitle", "All gists from "+fromUser)
		setData(ctx, "fromUser", fromUser)

		var count int64
		if err = models.DoesUserExists(fromUser, &count); err != nil {
			return errorRes(500, "Error fetching user", err)
		}

		if count == 0 {
			return notFound("User not found")
		}

		gists, err = models.GetAllGistsFromUser(fromUser, currentUserId, pageInt-1, sort, order)
	}
	if err != nil {
		return errorRes(500, "Error fetching gists", err)
	}

	if err = paginate(ctx, gists, pageInt, 10, "gists", fromUser, "&sort="+sort+"&order="+order); err != nil {
		return errorRes(404, "Page not found", nil)
	}

	return html(ctx, "all.html")
}

func gist(ctx echo.Context) error {
	gist := getData(ctx, "gist").(*models.Gist)
	userName := gist.User.Username
	gistName := gist.Uuid
	revision := ctx.Param("revision")

	if revision == "" {
		revision = "HEAD"
	}

	nbCommits := getData(ctx, "nbCommits")
	files := make(map[string]string)
	if nbCommits != "0" {
		filesStr, err := git.GetFilesOfRepository(userName, gistName, revision)
		if err != nil {
			return errorRes(500, "Error fetching files from repository", err)
		}
		for _, file := range filesStr {
			files[file], err = git.GetFileContent(userName, gistName, revision, file)
			if err != nil {
				return errorRes(500, "Error fetching file content from file "+file, err)
			}
		}
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

	nbCommits := getData(ctx, "nbCommits")
	commits := make([]*models.Commit, 0)
	if nbCommits != "0" {
		gitlogStr, err := git.GetLog(userName, gistName, strconv.Itoa((pageInt-1)*10))
		if err != nil {
			return errorRes(500, "Error fetching commits log", err)
		}

		gitlog := strings.Split(gitlogStr, "\n=commit ")
		for _, commitStr := range gitlog[1:] {
			logContent := strings.SplitN(commitStr, "\n", 3)

			header := strings.Split(logContent[0], ":")
			commitStruct := models.Commit{
				Hash:      header[0],
				Author:    header[1],
				Timestamp: header[2],
				Files:     make([]models.File, 0),
			}

			if len(logContent) > 2 {
				changed := strings.ReplaceAll(logContent[1], "(+)", "")
				changed = strings.ReplaceAll(changed, "(-)", "")
				commitStruct.Changed = changed
			}

			files := strings.Split(logContent[len(logContent)-1], "diff --git ")
			if len(files) > 1 {
				for _, fileStr := range files {
					content := strings.SplitN(fileStr, "\n@@", 2)
					if len(content) > 1 {
						header := strings.Split(content[0], "\n")
						commitStruct.Files = append(commitStruct.Files, models.File{Content: "@@" + content[1], Filename: header[len(header)-1][4:], OldFilename: header[len(header)-2][4:]})
					} else {
						// in case there is no content but a file renamed
						header := strings.Split(content[0], "\n")
						if len(header) > 3 {
							commitStruct.Files = append(commitStruct.Files, models.File{Content: "", Filename: header[3][10:], OldFilename: header[2][12:]})
						}
					}
				}
			}
			commits = append(commits, &commitStruct)
		}
	}

	if err := paginate(ctx, commits, pageInt, 10, "commits", userName+"/"+gistName+"/revisions"); err != nil {
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

	var gist *models.Gist

	if isCreate {
		gist = new(models.Gist)
		setData(ctx, "htmlTitle", "Create a new gist")
	} else {
		gist = getData(ctx, "gist").(*models.Gist)
		setData(ctx, "htmlTitle", "Edit "+gist.Title)
	}

	if err := ctx.Bind(gist); err != nil {
		return errorRes(400, "Cannot bind data", err)
	}

	gist.Files = make([]models.File, 0)
	for i := 0; i < len(ctx.Request().PostForm["content"]); i++ {
		name := ctx.Request().PostForm["name"][i]
		content := ctx.Request().PostForm["content"][i]

		if name == "" {
			name = "gistfile" + strconv.Itoa(i+1) + ".txt"
		}

		escapedValue, err := url.QueryUnescape(content)
		if err != nil {
			return errorRes(400, "Invalid character unescaped", err)
		}

		gist.Files = append(gist.Files, models.File{
			Filename: name,
			Content:  escapedValue,
		})
	}
	user := getUserLogged(ctx)
	gist.NbFiles = len(gist.Files)

	if isCreate {
		uuidGist, err := uuid.NewRandom()
		if err != nil {
			return errorRes(500, "Error creating an UUID", err)
		}
		gist.Uuid = strings.Replace(uuidGist.String(), "-", "", -1)

		gist.UserID = user.ID
	}

	if gist.Title == "" {
		if ctx.Request().PostForm["name"][0] == "" {
			gist.Title = "gist:" + gist.Uuid
		} else {
			gist.Title = ctx.Request().PostForm["name"][0]
		}
	}

	err = ctx.Validate(gist)
	if err != nil {
		addFlash(ctx, validationMessages(&err), "error")
		if isCreate {
			return html(ctx, "create.html")
		} else {
			files := make(map[string]string)
			filesStr, err := git.GetFilesOfRepository(gist.User.Username, gist.Uuid, "HEAD")
			if err != nil {
				return errorRes(500, "Error fetching files from repository", err)
			}
			for _, file := range filesStr {
				files[file], err = git.GetFileContent(gist.User.Username, gist.Uuid, "HEAD", file)
				if err != nil {
					return errorRes(500, "Error fetching file content from file "+file, err)
				}
			}

			setData(ctx, "files", files)
			return html(ctx, "edit.html")
		}
	}

	if len(gist.Files) > 0 {
		split := strings.Split(gist.Files[0].Content, "\n")
		if len(split) > 10 {
			gist.Preview = strings.Join(split[:10], "\n")
		} else {
			gist.Preview = gist.Files[0].Content
		}

		gist.PreviewFilename = gist.Files[0].Filename
	}

	if err = git.InitRepository(user.Username, gist.Uuid); err != nil {
		return errorRes(500, "Error creating the repository", err)
	}

	if err = git.CloneTmp(user.Username, gist.Uuid, gist.Uuid); err != nil {
		return errorRes(500, "Error cloning the repository", err)
	}

	for _, file := range gist.Files {
		if err = git.SetFileContent(gist.Uuid, file.Filename, file.Content); err != nil {
			return errorRes(500, "Error setting file content for file "+file.Filename, err)
		}
	}

	if err = git.AddAll(gist.Uuid); err != nil {
		return errorRes(500, "Error adding files to the repository", err)
	}

	if err = git.Commit(gist.Uuid); err != nil {
		return errorRes(500, "Error committing files to the local repository", err)
	}

	if err = git.Push(gist.Uuid); err != nil {
		return errorRes(500, "Error pushing the local repository", err)
	}

	if isCreate {
		if err = models.CreateGist(gist); err != nil {
			return errorRes(500, "Error creating the gist", err)
		}
	} else {
		if err = models.UpdateGist(gist); err != nil {
			return errorRes(500, "Error updating the gist", err)
		}
	}

	return redirect(ctx, "/"+user.Username+"/"+gist.Uuid)
}

func toggleVisibility(ctx echo.Context) error {
	var gist = getData(ctx, "gist").(*models.Gist)

	gist.Private = !gist.Private
	if err := models.UpdateGist(gist); err != nil {
		return errorRes(500, "Error updating this gist", err)
	}

	addFlash(ctx, "Gist visibility has been changed", "success")
	return redirect(ctx, "/"+gist.User.Username+"/"+gist.Uuid)
}

func deleteGist(ctx echo.Context) error {
	var gist = getData(ctx, "gist").(*models.Gist)

	err := git.DeleteRepository(gist.User.Username, gist.Uuid)
	if err != nil {
		return errorRes(500, "Error deleting the repository", err)
	}

	if err := models.DeleteGist(gist); err != nil {
		return errorRes(500, "Error deleting this gist", err)
	}

	addFlash(ctx, "Gist has been deleted", "success")
	return redirect(ctx, "/")
}

func like(ctx echo.Context) error {
	var gist = getData(ctx, "gist").(*models.Gist)
	currentUser := getUserLogged(ctx)

	hasLiked, err := models.UserHasLikedGist(currentUser, gist)
	if err != nil {
		return errorRes(500, "Error checking if user has liked a gist", err)
	}

	if hasLiked {
		err = models.RemoveUserLike(gist, getUserLogged(ctx))
	} else {
		err = models.AppendUserLike(gist, getUserLogged(ctx))
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

func rawFile(ctx echo.Context) error {
	gist := getData(ctx, "gist").(*models.Gist)
	fileContent, err := git.GetFileContent(
		gist.User.Username,
		gist.Uuid,
		ctx.Param("revision"),
		ctx.Param("file"))
	if err != nil {
		return errorRes(500, "Error getting file content", err)
	}

	filebytes := []byte(fileContent)

	if len(filebytes) == 0 {
		return notFound("File not found")
	}

	return plainText(ctx, 200, string(filebytes))
}

func edit(ctx echo.Context) error {
	var gist = getData(ctx, "gist").(*models.Gist)

	files := make(map[string]string)
	filesStr, err := git.GetFilesOfRepository(gist.User.Username, gist.Uuid, "HEAD")
	if err != nil {
		return errorRes(500, "Error fetching files from repository", err)
	}
	for _, file := range filesStr {
		files[file], err = git.GetFileContent(gist.User.Username, gist.Uuid, "HEAD", file)
		if err != nil {
			return errorRes(500, "Error fetching file content from file "+file, err)
		}
	}

	setData(ctx, "files", files)
	setData(ctx, "htmlTitle", "Edit "+gist.Title)

	return html(ctx, "edit.html")
}

func downloadZip(ctx echo.Context) error {
	var gist = getData(ctx, "gist").(*models.Gist)
	var revision = ctx.Param("revision")

	files := make(map[string]string)
	filesStr, err := git.GetFilesOfRepository(gist.User.Username, gist.Uuid, revision)
	if err != nil {
		return errorRes(500, "Error fetching files from repository", err)
	}

	for _, file := range filesStr {
		files[file], err = git.GetFileContent(gist.User.Username, gist.Uuid, revision, file)
		if err != nil {
			return errorRes(500, "Error fetching file content from file "+file, err)
		}
	}

	zipFile := new(bytes.Buffer)

	zipWriter := zip.NewWriter(zipFile)

	for fileName, fileContent := range files {
		f, err := zipWriter.Create(fileName)
		if err != nil {
			return errorRes(500, "Error adding a file the to the zip archive", err)
		}
		_, err = f.Write([]byte(fileContent))
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

	likers, err := models.GetUsersLikesForGists(gist, pageInt-1)
	if err != nil {
		return errorRes(500, "Error getting users who liked this gist", err)
	}

	if err = paginate(ctx, likers, pageInt, 30, "likers", gist.User.Username+"/"+gist.Uuid+"/likes"); err != nil {
		return errorRes(404, "Page not found", nil)
	}

	setData(ctx, "htmlTitle", "Likes for "+gist.Title)
	setData(ctx, "revision", "HEAD")
	return html(ctx, "likes.html")
}
