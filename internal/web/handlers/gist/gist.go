package gist

import (
	"bufio"
	"bytes"
	gojson "encoding/json"
	"fmt"
	"net/url"
	"strings"
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

	files, hasMoreFiles, err := gist.Files(revision, true)
	if _, ok := err.(*git.RevisionNotFoundError); ok {
		return ctx.NotFound("Revision not found")
	} else if err != nil {
		return ctx.ErrorRes(500, "Error fetching files", err)
	}

	renderedFiles := render.RenderFiles(files)

	ctx.SetData("page", "code")
	ctx.SetData("commit", revision)
	ctx.SetData("files", renderedFiles)
	ctx.SetData("hasMoreFiles", hasMoreFiles)
	ctx.SetData("revision", revision)
	ctx.SetData("htmlTitle", gist.Title)
	return ctx.Html("gist.html")
}

func GistJson(ctx *context.Context) error {
	gist := ctx.GetData("gist").(*db.Gist)

	var files []*git.File
	hasMoreFiles := false
	embedFile := ctx.QueryParam("file")

	if embedFile != "" {
		file, err := gist.File("HEAD", embedFile, true)
		if err != nil {
			return ctx.ErrorRes(500, "Error fetching file", err)
		}
		if file == nil {
			return ctx.NotFound(ctx.Tr("error.file_not_found"))
		}
		files = []*git.File{file}
	} else {
		var err error
		files, hasMoreFiles, err = gist.Files("HEAD", true)
		if err != nil {
			return ctx.ErrorRes(500, "Error fetching files", err)
		}
	}

	renderedFiles := render.RenderFiles(files)
	ctx.SetData("files", renderedFiles)
	ctx.SetData("hasMoreFiles", hasMoreFiles)

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

	jsBaseUrl, err := url.JoinPath(ctx.GetData("baseHttpUrl").(string), gist.User.Username, gist.Identifier()+".js")
	if err != nil {
		return ctx.ErrorRes(500, "Error joining js url", err)
	}

	// Build per-file and per-theme URL variants.
	fileQuery, themeSep := "", "?"
	if embedFile != "" {
		fileQuery = "?file=" + url.QueryEscape(embedFile)
		themeSep = "&"
	}
	jsUrl := jsBaseUrl + fileQuery
	baseHttpUrl := ctx.GetData("baseHttpUrl").(string)
	cssUrl, err := manifestCssUrl(baseHttpUrl, "ts/embed.ts")
	if err != nil {
		return ctx.ErrorRes(500, "Missing embed CSS in manifest", err)
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
			"html":     htmlbuf.String(),
			"css":      cssUrl,
			"js":       jsUrl,
			"js_dark":  jsUrl + themeSep + "dark",
			"js_light": jsUrl + themeSep + "light",
			"js_auto":  jsUrl + themeSep + "auto",
		},
	})
}

func GistJs(ctx *context.Context) error {
	params := ctx.QueryParams()
	_, hasDark := params["dark"]
	_, hasLight := params["light"]

	theme := "auto"
	autoMode := true
	if hasDark {
		ctx.SetData("dark", "dark")
		theme = "dark"
		autoMode = false
	} else if hasLight {
		theme = "light"
		autoMode = false
	}

	gist := ctx.GetData("gist").(*db.Gist)

	var files []*git.File
	hasMoreFiles := false
	embedFile := ctx.QueryParam("file")

	if embedFile != "" {
		file, err := gist.File("HEAD", embedFile, true)
		if err != nil {
			return ctx.ErrorRes(500, "Error fetching file", err)
		}
		if file == nil {
			return ctx.NotFound(ctx.Tr("error.file_not_found"))
		}
		files = []*git.File{file}
	} else {
		var err error
		files, hasMoreFiles, err = gist.Files("HEAD", true)
		if err != nil {
			return ctx.ErrorRes(500, "Error fetching files", err)
		}
	}

	renderedFiles := render.RenderFiles(files)
	ctx.SetData("files", renderedFiles)
	ctx.SetData("hasMoreFiles", hasMoreFiles)

	htmlbuf := bytes.Buffer{}
	w := bufio.NewWriter(&htmlbuf)
	if err := ctx.Echo().Renderer.Render(w, "gist_embed.html", ctx.DataMap(), ctx); err != nil {
		return err
	}
	_ = w.Flush()

	baseHttpUrl := ctx.GetData("baseHttpUrl").(string)
	cssUrl, err := manifestCssUrl(baseHttpUrl, "ts/embed.ts")
	if err != nil {
		return ctx.ErrorRes(500, "Missing embed CSS in manifest", err)
	}

	themeUrl, err := manifestCssUrl(baseHttpUrl, "ts/"+theme+".ts")
	if err != nil {
		return ctx.ErrorRes(500, "Missing theme CSS in manifest", err)
	}

	js, err := escapeJavaScriptContent(htmlbuf.String(), cssUrl, themeUrl, autoMode)
	if err != nil {
		return ctx.ErrorRes(500, "Error escaping JavaScript content", err)
	}
	ctx.Response().Header().Set("Cache-Control", "no-store")
	ctx.Response().Header().Set("Content-Type", "text/javascript")
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

// manifestCssUrl returns the full CSS URL for a vite manifest key (e.g. "ts/embed.ts").
// In dev mode (ManifestEntries is nil) it falls back to the vite dev server, deriving the
// CSS path from the TS key: "ts/embed.ts" → "http://localhost:16157/css/embed.css".
func manifestCssUrl(baseHttpUrl, key string) (string, error) {
	if context.ManifestEntries == nil {
		name := strings.TrimSuffix(strings.TrimPrefix(key, "ts/"), ".ts")
		return "http://localhost:16157/css/" + name + ".css", nil
	}
	entry, ok := context.ManifestEntries[key]
	if !ok || len(entry.Css) == 0 {
		return "", fmt.Errorf("no CSS entry for manifest key %q", key)
	}
	return url.JoinPath(baseHttpUrl, entry.Css[0])
}

func escapeJavaScriptContent(htmlContent, cssUrl, themeUrl string, autoMode bool) (string, error) {
	jsonContent, err := gojson.Marshal(htmlContent)
	if err != nil {
		return "", fmt.Errorf("failed to encode content: %w", err)
	}

	jsonCssUrl, err := gojson.Marshal(cssUrl)
	if err != nil {
		return "", fmt.Errorf("failed to encode CSS URL: %w", err)
	}

	jsonThemeUrl, err := gojson.Marshal(themeUrl)
	if err != nil {
		return "", fmt.Errorf("failed to encode Theme URL: %w", err)
	}

	jsonAutoMode, err := gojson.Marshal(autoMode)
	if err != nil {
		return "", fmt.Errorf("failed to encode auto mode: %w", err)
	}

	js := fmt.Sprintf(`
(function() {
    if (!customElements.get('opengist-embed')) {
        customElements.define('opengist-embed', class extends HTMLElement {
            constructor() {
                super();
                this.attachShadow({ mode: 'open' });
            }

            init(css1, css2, content, autoMode) {
                this.shadowRoot.innerHTML = %s
                    <style>
                        @import url(${css1});
                        @import url(${css2});
                        :host { display: block; all: initial; font-family: sans-serif; }
                    </style>
                    <div class="container">${content}</div>
                %s;
                if (autoMode) {
                    const mq = window.matchMedia('(prefers-color-scheme: dark)');
                    const htmlDiv = this.shadowRoot.querySelector('.html');
                    const applyTheme = () => htmlDiv && htmlDiv.classList.toggle('dark', mq.matches);
                    mq.addEventListener('change', applyTheme);
                    applyTheme();
                }
            }
        });
    }

    var currentScript = document.currentScript || (function() {
        var scripts = document.getElementsByTagName('script');
        return scripts[scripts.length - 1];
    })();

    const instance = document.createElement('opengist-embed');
    instance.init(%s, %s, %s, %s);
    currentScript.parentNode.insertBefore(instance, currentScript.nextSibling);
})();
`,
		"`",
		"`",
		string(jsonCssUrl),
		string(jsonThemeUrl),
		string(jsonContent),
		string(jsonAutoMode),
	)

	return js, nil
}
