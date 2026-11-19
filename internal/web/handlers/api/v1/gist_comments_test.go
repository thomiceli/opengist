package v1_test

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/web/handlers/api/v1/types"
	webtest "github.com/thomiceli/opengist/internal/web/test"
)

// postCommentAPI posts body as a comment on the gist uuid and returns the
// created comment.
func postCommentAPI(t *testing.T, s *webtest.Server, tok, uuid, body string) types.GistComment {
	var c types.GistComment
	s.APIRequestUnmarshal(t, "POST", "/api/gists/"+uuid+"/comments", tok, map[string]string{"body": body}, &c, 201)
	return c
}

func commentURI(uuid string, id uint) string {
	return "/api/gists/" + uuid + "/comments/" + strconv.FormatUint(uint64(id), 10)
}

func TestListComments_VisibilityAccess(t *testing.T) {
	s := setupGetGist(t)

	_, gPub, _, _ := s.CreateGistAs(t, "owner", "0")
	_, gUnl, _, _ := s.CreateGistAs(t, "owner", "1")
	_, gPriv, _, _ := s.CreateGistAs(t, "owner", "2")

	ownerTok := apiTokenFor(t, s, "owner", db.ReadPermission)
	ownerNoScope := apiTokenFor(t, s, "owner", db.NoPermission)
	otherTok := apiTokenFor(t, s, "other", db.ReadPermission)

	cases := []struct {
		name string
		uuid string
		tok  string
		want int
	}{
		{"public/anonymous", gPub.Uuid, "", 200},
		{"public/scoped other", gPub.Uuid, otherTok, 200},
		{"unlisted/anonymous", gUnl.Uuid, "", 200},
		{"unlisted/scoped other", gUnl.Uuid, otherTok, 200},
		{"private/anonymous", gPriv.Uuid, "", 404},
		{"private/no-scope owner", gPriv.Uuid, ownerNoScope, 404},
		{"private/scoped other", gPriv.Uuid, otherTok, 404},
		{"private/scoped owner", gPriv.Uuid, ownerTok, 200},
		{"unknown gist", "does-not-exist", ownerTok, 404},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s.APIRequest(t, "GET", "/api/gists/"+c.uuid+"/comments", c.tok, nil, c.want)
		})
	}
}

func TestCreateComment_API(t *testing.T) {
	s := setupGetGist(t)

	_, g, _, ident := s.CreateGistAs(t, "owner", "0")
	otherWrite := apiTokenFor(t, s, "other", db.ReadWritePermission)
	otherRead := apiTokenFor(t, s, "other", db.ReadPermission)

	t.Run("Created", func(t *testing.T) {
		w, raw := s.APIRequest(t, "POST", "/api/gists/"+g.Uuid+"/comments", otherWrite, map[string]string{"body": "Nice *gist*\r\n"}, 201)
		var c types.GistComment
		require.NoError(t, json.Unmarshal(raw, &c))

		require.NotZero(t, c.ID)
		require.Equal(t, "Nice *gist*", c.Body)
		require.Equal(t, "other", c.User.Username)
		require.True(t, strings.HasSuffix(c.URL, commentURI(g.Uuid, c.ID)), c.URL)
		require.True(t, strings.HasSuffix(c.HTMLUrl, "/owner/"+ident+"/comments/"+strconv.FormatUint(uint64(c.ID), 10)), c.HTMLUrl)
		require.Equal(t, c.CreatedAt, c.UpdatedAt)
		require.Equal(t, c.URL, w.Header().Get("Location"))

		var got types.Gist
		s.APIRequestUnmarshal(t, "GET", "/api/gists/"+g.Uuid, "", nil, &got, 200)
		require.Equal(t, 1, got.CommentCount)
	})

	t.Run("Anonymous", func(t *testing.T) {
		s.APIRequest(t, "POST", "/api/gists/"+g.Uuid+"/comments", "", map[string]string{"body": "hi"}, 401)
	})

	t.Run("ReadOnlyToken", func(t *testing.T) {
		s.APIRequest(t, "POST", "/api/gists/"+g.Uuid+"/comments", otherRead, map[string]string{"body": "hi"}, 403)
	})

	t.Run("InvalidBody", func(t *testing.T) {
		for name, body := range map[string]interface{}{
			"missing":    map[string]string{},
			"empty":      map[string]string{"body": ""},
			"whitespace": map[string]string{"body": " \n\t "},
			"too long":   map[string]string{"body": strings.Repeat("a", 3001)},
			"not json":   "{",
		} {
			t.Run(name, func(t *testing.T) {
				s.APIRequest(t, "POST", "/api/gists/"+g.Uuid+"/comments", otherWrite, body, 422)
			})
		}
	})

	t.Run("PrivateGist", func(t *testing.T) {
		_, gPriv, _, _ := s.CreateGistAs(t, "owner", "2")
		s.APIRequest(t, "POST", "/api/gists/"+gPriv.Uuid+"/comments", otherWrite, map[string]string{"body": "hi"}, 404)

		ownerWrite := apiTokenFor(t, s, "owner", db.ReadWritePermission)
		postCommentAPI(t, s, ownerWrite, gPriv.Uuid, "note to self")
	})

	t.Run("ArchivedGist", func(t *testing.T) {
		_, gArch, _, _ := s.CreateGistAs(t, "owner", "0")
		require.NoError(t, gArch.SetArchived(true))
		s.APIRequest(t, "POST", "/api/gists/"+gArch.Uuid+"/comments", otherWrite, map[string]string{"body": "hi"}, 403)
	})
}

func TestListAndGetComments_API(t *testing.T) {
	s := setupGetGist(t)

	_, g, _, _ := s.CreateGistAs(t, "owner", "0")
	_, gOther, _, _ := s.CreateGistAs(t, "owner", "0")
	ownerWrite := apiTokenFor(t, s, "owner", db.ReadWritePermission)
	otherWrite := apiTokenFor(t, s, "other", db.ReadWritePermission)

	first := postCommentAPI(t, s, otherWrite, g.Uuid, "first")
	second := postCommentAPI(t, s, ownerWrite, g.Uuid, "second")
	third := postCommentAPI(t, s, otherWrite, g.Uuid, "third")
	elsewhere := postCommentAPI(t, s, otherWrite, gOther.Uuid, "on another gist")

	t.Run("List", func(t *testing.T) {
		var list []types.GistComment
		w, raw := s.APIRequest(t, "GET", "/api/gists/"+g.Uuid+"/comments", "", nil, 200)
		require.NoError(t, json.Unmarshal(raw, &list))
		require.Len(t, list, 3)
		require.Equal(t, []uint{first.ID, second.ID, third.ID}, []uint{list[0].ID, list[1].ID, list[2].ID})
		require.Equal(t, "owner", list[1].User.Username)
		require.Equal(t, "3", w.Header().Get("X-Total"))
		require.Empty(t, w.Header().Get("Link"))
	})

	t.Run("Paginated", func(t *testing.T) {
		var list []types.GistComment
		w, raw := s.APIRequest(t, "GET", "/api/gists/"+g.Uuid+"/comments?per_page=2&page=2", "", nil, 200)
		require.NoError(t, json.Unmarshal(raw, &list))
		require.Len(t, list, 1)
		require.Equal(t, third.ID, list[0].ID)
		require.Equal(t, "2", w.Header().Get("X-Total-Pages"))
		require.Contains(t, w.Header().Get("Link"), `rel="prev"`)
		require.NotContains(t, w.Header().Get("Link"), `rel="next"`)
	})

	t.Run("Get", func(t *testing.T) {
		var c types.GistComment
		s.APIRequestUnmarshal(t, "GET", commentURI(g.Uuid, second.ID), "", nil, &c, 200)
		require.Equal(t, second, c)
	})

	t.Run("GetNotFound", func(t *testing.T) {
		s.APIRequest(t, "GET", commentURI(g.Uuid, elsewhere.ID), "", nil, 404)
		s.APIRequest(t, "GET", commentURI(g.Uuid, 999999), "", nil, 404)
		s.APIRequest(t, "GET", "/api/gists/"+g.Uuid+"/comments/abc", "", nil, 404)
	})
}

func TestUpdateComment_API(t *testing.T) {
	s := setupGetGist(t)

	_, g, _, _ := s.CreateGistAs(t, "owner", "0")
	ownerWrite := apiTokenFor(t, s, "owner", db.ReadWritePermission)
	otherWrite := apiTokenFor(t, s, "other", db.ReadWritePermission)
	adminWrite := apiTokenFor(t, s, "admin", db.ReadWritePermission)

	c := postCommentAPI(t, s, otherWrite, g.Uuid, "first version")

	t.Run("NotAuthor", func(t *testing.T) {
		// neither the gist owner nor an admin can rewrite someone's comment
		s.APIRequest(t, "PATCH", commentURI(g.Uuid, c.ID), ownerWrite, map[string]string{"body": "hacked"}, 403)
		s.APIRequest(t, "PATCH", commentURI(g.Uuid, c.ID), adminWrite, map[string]string{"body": "hacked"}, 403)
		s.APIRequest(t, "PATCH", commentURI(g.Uuid, c.ID), "", map[string]string{"body": "hacked"}, 401)
	})

	t.Run("InvalidBody", func(t *testing.T) {
		s.APIRequest(t, "PATCH", commentURI(g.Uuid, c.ID), otherWrite, map[string]string{"body": "  "}, 422)
	})

	t.Run("Author", func(t *testing.T) {
		var updated types.GistComment
		s.APIRequestUnmarshal(t, "PATCH", commentURI(g.Uuid, c.ID), otherWrite, map[string]string{"body": "second version"}, &updated, 200)
		require.Equal(t, c.ID, updated.ID)
		require.Equal(t, "second version", updated.Body)
		require.Equal(t, "other", updated.User.Username)

		var got types.GistComment
		s.APIRequestUnmarshal(t, "GET", commentURI(g.Uuid, c.ID), "", nil, &got, 200)
		require.Equal(t, "second version", got.Body)
	})

	t.Run("ArchivedGist", func(t *testing.T) {
		require.NoError(t, g.SetArchived(true))
		defer func() { require.NoError(t, g.SetArchived(false)) }()
		s.APIRequest(t, "PATCH", commentURI(g.Uuid, c.ID), otherWrite, map[string]string{"body": "third version"}, 403)
	})
}

func TestDeleteComment_API(t *testing.T) {
	s := setupGetGist(t)
	s.Register(t, "third")
	s.Logout()

	_, g, _, _ := s.CreateGistAs(t, "owner", "0")
	ownerWrite := apiTokenFor(t, s, "owner", db.ReadWritePermission)
	otherWrite := apiTokenFor(t, s, "other", db.ReadWritePermission)
	otherRead := apiTokenFor(t, s, "other", db.ReadPermission)
	adminWrite := apiTokenFor(t, s, "admin", db.ReadWritePermission)
	thirdWrite := apiTokenFor(t, s, "third", db.ReadWritePermission)

	byAuthor := postCommentAPI(t, s, otherWrite, g.Uuid, "deleted by its author")
	byOwner := postCommentAPI(t, s, otherWrite, g.Uuid, "deleted by the gist owner")
	byAdmin := postCommentAPI(t, s, otherWrite, g.Uuid, "deleted by an admin")

	requireCount := func(t *testing.T, want int) {
		var got types.Gist
		s.APIRequestUnmarshal(t, "GET", "/api/gists/"+g.Uuid, "", nil, &got, 200)
		require.Equal(t, want, got.CommentCount)
	}
	requireCount(t, 3)

	t.Run("Forbidden", func(t *testing.T) {
		s.APIRequest(t, "DELETE", commentURI(g.Uuid, byAuthor.ID), thirdWrite, nil, 403)
		s.APIRequest(t, "DELETE", commentURI(g.Uuid, byAuthor.ID), otherRead, nil, 403)
		s.APIRequest(t, "DELETE", commentURI(g.Uuid, byAuthor.ID), "", nil, 401)
		requireCount(t, 3)
	})

	t.Run("Author", func(t *testing.T) {
		s.APIRequest(t, "DELETE", commentURI(g.Uuid, byAuthor.ID), otherWrite, nil, 204)
		s.APIRequest(t, "GET", commentURI(g.Uuid, byAuthor.ID), "", nil, 404)
		s.APIRequest(t, "DELETE", commentURI(g.Uuid, byAuthor.ID), otherWrite, nil, 404)
		requireCount(t, 2)
	})

	t.Run("GistOwner", func(t *testing.T) {
		s.APIRequest(t, "DELETE", commentURI(g.Uuid, byOwner.ID), ownerWrite, nil, 204)
		requireCount(t, 1)
	})

	t.Run("Admin", func(t *testing.T) {
		s.APIRequest(t, "DELETE", commentURI(g.Uuid, byAdmin.ID), adminWrite, nil, 204)
		requireCount(t, 0)
	})
}

func TestComments_API_Disabled(t *testing.T) {
	s := setupGetGist(t)

	_, g, _, _ := s.CreateGistAs(t, "owner", "0")
	otherWrite := apiTokenFor(t, s, "other", db.ReadWritePermission)
	c := postCommentAPI(t, s, otherWrite, g.Uuid, "hello")

	config.C.DisableComments = true
	defer func() { config.C.DisableComments = false }()

	s.APIRequest(t, "GET", "/api/gists/"+g.Uuid+"/comments", "", nil, 403)
	s.APIRequest(t, "POST", "/api/gists/"+g.Uuid+"/comments", otherWrite, map[string]string{"body": "hi"}, 403)
	s.APIRequest(t, "GET", commentURI(g.Uuid, c.ID), "", nil, 403)
	s.APIRequest(t, "PATCH", commentURI(g.Uuid, c.ID), otherWrite, map[string]string{"body": "hi"}, 403)
	s.APIRequest(t, "DELETE", commentURI(g.Uuid, c.ID), otherWrite, nil, 403)
}
