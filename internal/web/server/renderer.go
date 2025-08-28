package server

import (
	gojson "encoding/json"
	"errors"
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/git"
	"github.com/thomiceli/opengist/internal/index"
	"github.com/thomiceli/opengist/internal/web/context"
	"github.com/thomiceli/opengist/internal/web/handlers"
	"github.com/thomiceli/opengist/public"
	"github.com/thomiceli/opengist/templates"
	htmlpkg "html"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Template struct {
	templates *template.Template
}

func (t *Template) Render(w io.Writer, name string, data interface{}, _ echo.Context) error {
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
		"isCsv": func(i string) bool {
			return strings.ToLower(filepath.Ext(i)) == ".csv"
		},
		"isJupyter": func(i string) bool {
			return strings.ToLower(filepath.Ext(i)) == ".ipynb"
		},
		"isSvg": func(i string) bool {
			return strings.ToLower(filepath.Ext(i)) == ".svg"
		},
		"csvFile": func(file *git.File) *git.CsvFile {
			if strings.ToLower(filepath.Ext(file.Filename)) != ".csv" {
				return nil
			}

			csvFile, err := git.ParseCsv(file)
			if err != nil {
				return nil
			}

			return csvFile
		},
		"httpStatusText": http.StatusText,
		"loadedTime": func(startTime time.Time) string {
			return fmt.Sprint(time.Since(startTime).Nanoseconds()/1e6) + "ms"
		},
		"slug": func(s string) string {
			return strings.Trim(re.ReplaceAllString(strings.ToLower(s), "-"), "-")
		},
		"avatarUrl": func(user *db.User, noGravatar bool) string {
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
			content, metadata := handlers.ParseSearchQueryStr(input)

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
		"hexToRgb": func(hex string) string {
			h, _ := strconv.ParseUint(strings.TrimPrefix(hex, "#"), 16, 32)
			return fmt.Sprintf("%d, %d, %d,", (h>>16)&0xFF, (h>>8)&0xFF, h&0xFF)
		},
	}

	t := template.Must(template.New("t").Funcs(fm).ParseFS(templates.Files, "*/*.html"))
	customPattern := filepath.Join(config.GetHomeDir(), "custom", "*.html")
	matches, err := filepath.Glob(customPattern)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to check for custom templates")
	}
	if len(matches) > 0 {
		t, err = t.ParseGlob(customPattern)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to parse custom templates")
		}
	}
	s.echo.Renderer = &Template{
		templates: t,
	}
}

func (s *Server) parseManifestEntries() {
	file, err := public.Files.Open("manifest.json")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to open manifest.json")
	}
	byteValue, err := io.ReadAll(file)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to read manifest.json")
	}
	if err = gojson.Unmarshal(byteValue, &context.ManifestEntries); err != nil {
		log.Fatal().Err(err).Msg("Failed to unmarshal manifest.json")
	}
}
