package gist

import (
	"bufio"
	"bytes"
	gojson "encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/git"
	"github.com/thomiceli/opengist/internal/render"
	"github.com/thomiceli/opengist/internal/web/context"
)

func GistIndex(ctx *context.Context) error {
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

	renderedFiles := render.RenderFiles(files)
	fmt.Println(len(renderedFiles))

	ctx.SetData("page", "code")
	ctx.SetData("commit", revision)
	ctx.SetData("files", renderedFiles)
	ctx.SetData("revision", revision)
	ctx.SetData("htmlTitle", gist.Title)
	return ctx.Html("gist.html")
}

func GistJson(ctx *context.Context) error {
	gist := ctx.GetData("gist").(*db.Gist)
	files, err := gist.Files("HEAD", true)
	if err != nil {
		return ctx.ErrorRes(500, "Error fetching files", err)
	}

	renderedFiles := render.RenderFiles(files)
	ctx.SetData("files", renderedFiles)

	topics, err := gist.GetTopics()
	if err != nil {
		return ctx.ErrorRes(500, "Error fetching topics for gist", err)
	}

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

	cssUrl, err := url.JoinPath(ctx.GetData("baseHttpUrl").(string), context.ManifestEntries["embed.css"].File)
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
		"topics":      topics,
		"embed": map[string]string{
			"html":    htmlbuf.String(),
			"css":     cssUrl,
			"js":      jsUrl,
			"js_dark": jsUrl + "?dark",
		},
	})
}

func GistJs(ctx *context.Context) error {
	if _, exists := ctx.QueryParams()["dark"]; exists {
		ctx.SetData("dark", "dark")
	}

	gist := ctx.GetData("gist").(*db.Gist)
	files, err := gist.Files("HEAD", true)
	if err != nil {
		return ctx.ErrorRes(500, "Error fetching files", err)
	}

	renderedFiles := render.RenderFiles(files)
	ctx.SetData("files", renderedFiles)

	htmlbuf := bytes.Buffer{}
	w := bufio.NewWriter(&htmlbuf)
	if err = ctx.Echo().Renderer.Render(w, "gist_embed.html", ctx.DataMap(), ctx); err != nil {
		return err
	}
	_ = w.Flush()

	cssUrl, err := url.JoinPath(ctx.GetData("baseHttpUrl").(string), context.ManifestEntries["embed.css"].File)
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

func Preview(ctx *context.Context) error {
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
