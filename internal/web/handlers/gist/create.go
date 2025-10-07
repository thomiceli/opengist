package gist

import (
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/git"
	"github.com/thomiceli/opengist/internal/i18n"
	"github.com/thomiceli/opengist/internal/validator"
	"github.com/thomiceli/opengist/internal/web/context"
)

func Create(ctx *context.Context) error {
	ctx.SetData("htmlTitle", ctx.TrH("gist.new.create-a-new-gist"))
	return ctx.Html("create.html")
}

func ProcessCreate(ctx *context.Context) error {
	isCreate := ctx.Request().URL.Path == "/"

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

	names := ctx.Request().PostForm["name"]
	contents := ctx.Request().PostForm["content"]

	// Process files from text editors
	for i, content := range contents {
		if content == "" {
			continue
		}
		name := names[i]
		if name == "" {
			fileCounter += 1
			name = "gistfile" + strconv.Itoa(fileCounter) + ".txt"
		}

		escapedValue, err := url.PathUnescape(content)
		if err != nil {
			return ctx.ErrorRes(400, ctx.Tr("error.invalid-character-unescaped"), err)
		}

		dto.Files = append(dto.Files, db.FileDTO{
			Filename: strings.TrimSpace(name),
			Content:  escapedValue,
		})
	}

	// Process uploaded files from UUID arrays
	fileUUIDs := ctx.Request().PostForm["uploadedfile_uuid"]
	fileFilenames := ctx.Request().PostForm["uploadedfile_filename"]
	if len(fileUUIDs) == len(fileFilenames) {
		for i, fileUUID := range fileUUIDs {
			filePath := filepath.Join(filepath.Join(config.GetHomeDir(), "uploads"), fileUUID)

			if _, err := os.Stat(filePath); err != nil {
				continue
			}

			dto.Files = append(dto.Files, db.FileDTO{
				Filename:   fileFilenames[i],
				SourcePath: filePath,
				Content:    "", // Empty since we're using SourcePath
			})
		}
	}

	// Process binary file operations (edit mode)
	binaryOldNames := ctx.Request().PostForm["binary_old_name"]
	binaryNewNames := ctx.Request().PostForm["binary_new_name"]
	if len(binaryOldNames) == len(binaryNewNames) {
		for i, oldName := range binaryOldNames {
			newName := binaryNewNames[i]

			if newName == "" { // deletion
				continue
			}

			if !isCreate {
				gistOld := ctx.GetData("gist").(*db.Gist)

				fileContent, _, err := git.GetFileContent(gistOld.User.Username, gistOld.Uuid, "HEAD", oldName, false)
				if err != nil {
					continue
				}

				dto.Files = append(dto.Files, db.FileDTO{
					Filename: newName,
					Content:  fileContent,
					Binary:   true,
				})
			}
		}
	}
	ctx.SetData("dto", dto)

	err = ctx.Validate(dto)
	if err != nil {
		ctx.AddFlash(validator.ValidationMessages(&err, ctx.GetData("locale").(*i18n.Locale)), "error")
		if isCreate {
			return ctx.HtmlWithCode(400, "create.html")
		} else {
			files, err := gist.Files("HEAD", false)
			if err != nil {
				return ctx.ErrorRes(500, "Error fetching files", err)
			}
			ctx.SetData("files", files)
			return ctx.HtmlWithCode(400, "edit.html")
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
		gist.Uuid = strings.ReplaceAll(uuidGist.String(), "-", "")

		gist.UserID = user.ID
		gist.User = *user
	}

	if gist.Title == "" {
		if dto.Files[0].Filename == "" {
			gist.Title = "gist:" + gist.Uuid
		} else {
			gist.Title = dto.Files[0].Filename
		}
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
	gist.UpdateLanguages()
	if err = gist.UpdatePreviewAndCount(true); err != nil {
		return ctx.ErrorRes(500, "Error updating preview and count", err)
	}

	return ctx.RedirectTo("/" + user.Username + "/" + gist.Identifier())
}
