package gist

import (
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/web/context"
)

const maxSidebarFiles = 50

func setSidebarFiles(ctx *context.Context, fileNames []string) {
	if len(fileNames) > maxSidebarFiles {
		fileNames = fileNames[:maxSidebarFiles]
	}
	ctx.SetData("sidebarFiles", fileNames)
}

// loadSidebarFiles adds a lightweight filename-only list for the persistent
// gist navigation. Full file contents are loaded only by the code view.
func loadSidebarFiles(ctx *context.Context, gist *db.Gist) error {
	fileNames, err := gist.FileNames("HEAD")
	if err != nil {
		return ctx.ErrorRes(500, "Error fetching file names", err)
	}

	setSidebarFiles(ctx, fileNames)
	return nil
}
