package server

import (
	gojson "encoding/json"
	"errors"
	"fmt"
	htmlpkg "html"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/index"
	"github.com/thomiceli/opengist/internal/web/context"
	"github.com/thomiceli/opengist/internal/web/handlers"
	"github.com/thomiceli/opengist/public"
	publicold "github.com/thomiceli/opengist/public-old"
	"github.com/thomiceli/opengist/templates"
	templatesold "github.com/thomiceli/opengist/templates-old"
)

type Template struct {
	templates *template.Template
	// pages holds the new layout-based templates, keyed by file name (e.g. "all.html").
	// Each entry is a clone of the base layout with that page's "content" block parsed in.
	pages  map[string]*template.Template
	legacy *template.Template
}

func (t *Template) Render(w io.Writer, name string, data interface{}, ctx echo.Context) error {
	if values, ok := data.(echo.Map); ok {
		values["uiRedirectUrl"] = ctx.Request().URL.RequestURI()
	}

	// Embeds are not part of the navigable web interface and use the new embed
	// stylesheet, so keep rendering them with their matching new template.
	if usesLegacyUI(ctx) && name != "gist_embed.html" && t.legacy != nil {
		legacyName := name
		if name == "settings_authentication.html" {
			legacyName = "settings_mfa.html"
		}
		if t.legacy.Lookup(legacyName) != nil {
			return t.legacy.ExecuteTemplate(w, legacyName, data)
		}
	}

	if tmpl, ok := t.pages[name]; ok {
		return tmpl.ExecuteTemplate(w, "base", data)
	}
	return t.templates.ExecuteTemplate(w, name, data)
}

var re = regexp.MustCompile("[^a-z0-9]+")

func (s *Server) setFuncMap() {
	fm := template.FuncMap{
		"split":     strings.Split,
		"indexByte": strings.IndexByte,
		"toInt": func(i string) int {
			val, _ := strconv.Atoi(i)
			return val
		},
		"inc": func(i int) int {
			return i + 1
		},
		"splitGit": func(i string) []string {
			return strings.FieldsFunc(i, func(r rune) bool {
				return r == ',' || r == ' '
			})
		},
		"lines": func(i string) []string {
			return strings.Split(i, "\n")
		},
		"isMarkdown": func(i string) bool {
			return strings.ToLower(filepath.Ext(i)) == ".md"
		},
		"isMermaid": func(i string) bool {
			return strings.ToLower(filepath.Ext(i)) == ".mmd"
		},
		"isJupyter": func(i string) bool {
			return strings.ToLower(filepath.Ext(i)) == ".ipynb"
		},
		"httpStatusText": http.StatusText,
		"loadedTime": func(startTime time.Time) string {
			return fmt.Sprint(time.Since(startTime).Nanoseconds()/1e6) + "ms"
		},
		"slug": func(s string) string {
			return strings.Trim(re.ReplaceAllString(strings.ToLower(s), "-"), "-")
		},
		"avatarUrl": func(user *db.User, noGravatar bool) string {
			if user.HasUploadedAvatar() {
				return fmt.Sprintf("%s/avatar/%s", config.C.ExternalUrl, user.AvatarURL)
			}

			if user.AvatarURL != "" {
				return user.AvatarURL
			}

			if user.MD5Hash != "" && !noGravatar {
				return "https://www.gravatar.com/avatar/" + user.MD5Hash + "?d=identicon&s=200"
			}

			return ""
		},
		"shouldGenerateAvatar": func(user *db.User, noGravatar bool) bool {
			if user == nil {
				return true
			}
			return user.AvatarURL == "" && (user.MD5Hash == "" || noGravatar)
		},
		"asset": func(file string) string {
			if s.dev {
				return "http://localhost:16157/" + file
			}
			return config.C.ExternalUrl + "/" + context.ManifestEntries[file].File
		},
		"assetCss": func(file string) string {
			if s.dev {
				return "http://localhost:16157/" + file
			}
			return config.C.ExternalUrl + "/" + context.ManifestEntries[file].Css[0]
		},
		"custom": func(file string) string {
			assetpath, err := url.JoinPath("/", "assets", file)
			if err != nil {
				log.Error().Err(err).Msgf("Failed to join path for custom file %s", file)
			}
			return config.C.ExternalUrl + assetpath
		},
		"dev": func() bool {
			return s.dev
		},
		"visibilityStr": func(visibility db.Visibility, lowercase bool) string {
			s := "Public"
			switch visibility {
			case 1:
				s = "Unlisted"
			case 2:
				s = "Private"
			}

			if lowercase {
				return strings.ToLower(s)
			}
			return s
		},
		"unescape": htmlpkg.UnescapeString,
		"join": func(s ...string) string {
			return strings.Join(s, "")
		},
		"toStr": func(i interface{}) string {
			return fmt.Sprint(i)
		},
		"safe": func(s string) template.HTML {
			return template.HTML(s)
		},
		"dict": func(values ...interface{}) (map[string]interface{}, error) {
			if len(values)%2 != 0 {
				return nil, errors.New("invalid dict call")
			}
			dict := make(map[string]interface{})
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil, errors.New("dict keys must be strings")
				}
				dict[key] = values[i+1]
			}
			return dict, nil
		},
		"addMetadataToSearchQuery": func(input, key, value string) string {
			metadata := handlers.ParseSearchQueryStr(input)
			// extract free-text content (stored under "all") and remove it from metadata
			content := metadata["all"]
			delete(metadata, "all")

			metadata[key] = value

			var resultBuilder strings.Builder
			resultBuilder.WriteString(content)

			for k, v := range metadata {
				resultBuilder.WriteString(" ")
				resultBuilder.WriteString(k)
				resultBuilder.WriteString(":")
				resultBuilder.WriteString(v)
			}

			return strings.TrimSpace(resultBuilder.String())
		},
		"indexEnabled": index.IndexEnabled,
		"isUrl": func(s string) bool {
			_, err := url.ParseRequestURI(s)
			return err == nil
		},
		"topicsToStr": func(topics []db.GistTopic) string {
			str := ""
			for i, topic := range topics {
				if i > 0 {
					str += " "
				}
				str += topic.Topic
			}
			return str
		},
		// ogText collapses all whitespace (newlines, tabs, multiple spaces) into
		// single spaces and truncates to maxRunes runes, appending an ellipsis when
		// truncated. Used to build clean values for Open Graph / Twitter meta tags
		// from multi-line gist descriptions or code previews.
		"ogText": func(s string, maxRunes int) string {
			collapsed := strings.Join(strings.Fields(s), " ")
			runes := []rune(collapsed)
			if len(runes) <= maxRunes {
				return collapsed
			}
			return string(runes[:maxRunes]) + "\u2026"
		},
		"hexToRgb": func(hex string) string {
			h, _ := strconv.ParseUint(strings.TrimPrefix(hex, "#"), 16, 32)
			return fmt.Sprintf("%d, %d, %d,", (h>>16)&0xFF, (h>>8)&0xFF, h&0xFF)
		},
		"humanTimeDiff": func(t int64) string {
			return humanize.Time(time.Unix(t, 0))
		},
		"humanTimeDiffStr": func(timestamp string) string {
			t, _ := strconv.ParseInt(timestamp, 10, 64)
			return humanize.Time(time.Unix(t, 0))
		},
		"humanDate": func(t int64) string {
			return time.Unix(t, 0).Format("02/01/2006 15:04")
		},
		"humanDateOnly": func(t int64) string {
			return time.Unix(t, 0).Format("02/01/2006")
		},
		"mainTheme": func(theme *db.UserStyleDTO) string {
			if theme == nil {
				return "auto"
			}

			if theme.Theme == "" {
				return "auto"
			}

			return theme.Theme
		},
	}

	base := template.Must(template.New("base").Funcs(fm).ParseFS(templates.Files, "layouts/*.html", "partials/*.html"))
	pagePaths, err := fs.Glob(templates.Files, "pages/*.html")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to glob new page templates")
	}
	pages := make(map[string]*template.Template, len(pagePaths))
	for _, p := range pagePaths {
		cloned := template.Must(base.Clone())
		pages[filepath.Base(p)] = template.Must(cloned.ParseFS(templates.Files, p))
	}

	legacyFM := make(template.FuncMap, len(fm))
	for name, fn := range fm {
		legacyFM[name] = fn
	}
	legacyFM["asset"] = func(file string) string {
		entry, ok := s.legacyManifestEntries[file]
		if !ok {
			return config.C.ExternalUrl + "/assets-old/" + file
		}
		return config.C.ExternalUrl + "/assets-old/" + entry.File
	}
	legacyFM["assetCss"] = func(file string) string {
		entry, ok := s.legacyManifestEntries[file]
		if !ok || len(entry.Css) == 0 {
			return config.C.ExternalUrl + "/assets-old/" + file
		}
		return config.C.ExternalUrl + "/assets-old/" + entry.Css[0]
	}
	// The legacy bundle is built once and served by the backend in development;
	// only the new UI is attached to the Vite development server.
	legacyFM["dev"] = func() bool { return false }
	legacy := template.Must(template.New("legacy").Funcs(legacyFM).ParseFS(templatesold.Files, "*/*.html"))

	s.echo.Renderer = &Template{
		templates: base,
		pages:     pages,
		legacy:    legacy,
	}
}

func (s *Server) parseManifestEntries() {
	entries, err := parseManifest(public.Files)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load manifest.json")
	}
	context.ManifestEntries = entries
}

func (s *Server) parseLegacyManifestEntries() {
	entries, err := parseManifest(publicold.Files)
	if err != nil {
		if s.dev {
			log.Warn().Err(err).Msg("Failed to load legacy UI manifest; run npm run build:old")
			return
		}
		log.Fatal().Err(err).Msg("Failed to load legacy UI manifest.json")
	}
	s.legacyManifestEntries = entries
}

func parseManifest(files fs.FS) (map[string]context.Asset, error) {
	file, err := files.Open(".vite/manifest.json")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	byteValue, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	entries := make(map[string]context.Asset)
	if err = gojson.Unmarshal(byteValue, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}
