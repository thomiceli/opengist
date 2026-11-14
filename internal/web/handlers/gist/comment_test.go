package gist_test

import (
	"io"
	"net/url"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/db"
	webtest "github.com/thomiceli/opengist/internal/web/test"
)

func postComment(t *testing.T, s *webtest.Server, username, identifier, content string, expectedCode int) *db.GistComment {
	s.Request(t, "POST", "/"+username+"/"+identifier+"/comments", url.Values{"content": {content}}, expectedCode)

	gist, err := db.GetGist(username, identifier)
	require.NoError(t, err)
	comments, err := gist.GetComments()
	require.NoError(t, err)
	if len(comments) == 0 {
		return nil
	}
	return comments[len(comments)-1]
}

func commentPath(username, identifier string, comment *db.GistComment) string {
	return "/" + username + "/" + identifier + "/comments/" + strconv.FormatUint(uint64(comment.ID), 10)
}

func requireNbComments(t *testing.T, username, identifier string, expected int) {
	gist, err := db.GetGist(username, identifier)
	require.NoError(t, err)
	require.Equal(t, expected, gist.NbComments)
	count, err := gist.CountComments()
	require.NoError(t, err)
	require.Equal(t, int64(expected), count)
}

func TestCreateComment(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	s.Register(t, "thomas")
	s.Register(t, "alice")

	t.Run("Create", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "0")

		s.Login(t, "alice")
		resp := s.Request(t, "POST", "/"+username+"/"+identifier+"/comments", url.Values{"content": {"**Nice** gist!\r\nThanks\r\n\r\n"}}, 302)

		gist, err := db.GetGist(username, identifier)
		require.NoError(t, err)
		comments, err := gist.GetComments()
		require.NoError(t, err)
		require.Len(t, comments, 1)
		require.Equal(t, "**Nice** gist!\nThanks", comments[0].Content)
		require.Equal(t, "alice", comments[0].User.Username)
		require.False(t, comments[0].IsEdited())
		require.Equal(t, "/"+username+"/"+identifier+"#comment-"+strconv.FormatUint(uint64(comments[0].ID), 10), resp.Header.Get("Location"))
		requireNbComments(t, username, identifier, 1)

		resp = s.Request(t, "GET", "/"+username+"/"+identifier, nil, 200)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Contains(t, string(body), `id="comment-`+strconv.FormatUint(uint64(comments[0].ID), 10)+`"`)
		require.Contains(t, string(body), "<strong>Nice</strong> gist!")
	})

	t.Run("RendersSafeMarkdown", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "0")

		s.Login(t, "alice")
		postComment(t, s, username, identifier, `<script>alert(1)</script> [link](javascript:alert(1))`, 302)

		resp := s.Request(t, "GET", "/"+username+"/"+identifier, nil, 200)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.NotContains(t, string(body), "<script>alert(1)</script>")
		require.NotContains(t, string(body), `href="javascript:alert(1)"`)
	})

	t.Run("EmptyContent", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "0")

		s.Login(t, "alice")
		resp := s.Request(t, "POST", "/"+username+"/"+identifier+"/comments", url.Values{"content": {" \r\n\t "}}, 302)
		require.Equal(t, "/"+username+"/"+identifier+"#comments", resp.Header.Get("Location"))
		requireNbComments(t, username, identifier, 0)
	})

	t.Run("TooLong", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "0")

		s.Login(t, "alice")
		long := make([]byte, 65536)
		for i := range long {
			long[i] = 'a'
		}
		s.Request(t, "POST", "/"+username+"/"+identifier+"/comments", url.Values{"content": {string(long)}}, 302)
		requireNbComments(t, username, identifier, 0)
	})

	t.Run("NoAuth", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "0")

		s.Logout()
		resp := s.Request(t, "POST", "/"+username+"/"+identifier+"/comments", url.Values{"content": {"hello"}}, 302)
		require.Equal(t, "/-/all", resp.Header.Get("Location"))
		requireNbComments(t, username, identifier, 0)
	})

	t.Run("UnlistedGist", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "1")

		s.Login(t, "alice")
		require.NotNil(t, postComment(t, s, username, identifier, "hello", 302))
		requireNbComments(t, username, identifier, 1)
	})

	t.Run("PrivateGist", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "2")

		s.Login(t, "alice")
		s.Request(t, "POST", "/"+username+"/"+identifier+"/comments", url.Values{"content": {"hello"}}, 404)
		requireNbComments(t, username, identifier, 0)

		s.Login(t, "thomas")
		require.NotNil(t, postComment(t, s, username, identifier, "note to self", 302))
		requireNbComments(t, username, identifier, 1)
	})

	t.Run("ArchivedGist", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "0")

		s.Login(t, "thomas")
		s.Request(t, "POST", "/"+username+"/"+identifier+"/archive", nil, 302)

		s.Login(t, "alice")
		s.Request(t, "POST", "/"+username+"/"+identifier+"/comments", url.Values{"content": {"hello"}}, 403)
		requireNbComments(t, username, identifier, 0)
	})

	t.Run("CommentsDisabled", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "0")

		s.Login(t, "alice")
		postComment(t, s, username, identifier, "visible before disabling", 302)

		require.NoError(t, db.UpdateSetting(db.SettingDisableComments, "1"))
		defer func() {
			require.NoError(t, db.UpdateSetting(db.SettingDisableComments, "0"))
		}()

		s.Request(t, "POST", "/"+username+"/"+identifier+"/comments", url.Values{"content": {"hello"}}, 403)
		requireNbComments(t, username, identifier, 1)

		resp := s.Request(t, "GET", "/"+username+"/"+identifier, nil, 200)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.NotContains(t, string(body), "visible before disabling")
		require.NotContains(t, string(body), `id="comments"`)
	})
}

func TestEditComment(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	s.Register(t, "thomas")
	s.Register(t, "alice")
	s.Register(t, "bob")

	t.Run("Author", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "0")

		s.Login(t, "alice")
		comment := postComment(t, s, username, identifier, "first version", 302)

		resp := s.Request(t, "POST", commentPath(username, identifier, comment)+"/edit", url.Values{"content": {"second version"}}, 302)
		require.Equal(t, "/"+username+"/"+identifier+"#comment-"+strconv.FormatUint(uint64(comment.ID), 10), resp.Header.Get("Location"))

		edited, err := db.GetGistComment(comment.GistID, comment.ID)
		require.NoError(t, err)
		require.Equal(t, "second version", edited.Content)
		requireNbComments(t, username, identifier, 1)
	})

	t.Run("EmptyContent", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "0")

		s.Login(t, "alice")
		comment := postComment(t, s, username, identifier, "first version", 302)
		s.Request(t, "POST", commentPath(username, identifier, comment)+"/edit", url.Values{"content": {"  "}}, 302)

		edited, err := db.GetGistComment(comment.GistID, comment.ID)
		require.NoError(t, err)
		require.Equal(t, "first version", edited.Content)
	})

	t.Run("OnlyAuthor", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "0")

		s.Login(t, "alice")
		comment := postComment(t, s, username, identifier, "first version", 302)

		// neither another user, nor the gist owner (also the admin here) can edit it
		s.Login(t, "bob")
		s.Request(t, "POST", commentPath(username, identifier, comment)+"/edit", url.Values{"content": {"hacked"}}, 403)
		s.Login(t, "thomas")
		s.Request(t, "POST", commentPath(username, identifier, comment)+"/edit", url.Values{"content": {"hacked"}}, 403)

		edited, err := db.GetGistComment(comment.GistID, comment.ID)
		require.NoError(t, err)
		require.Equal(t, "first version", edited.Content)
	})

	t.Run("ArchivedGist", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "0")

		s.Login(t, "alice")
		comment := postComment(t, s, username, identifier, "first version", 302)

		s.Login(t, "thomas")
		s.Request(t, "POST", "/"+username+"/"+identifier+"/archive", nil, 302)

		s.Login(t, "alice")
		s.Request(t, "POST", commentPath(username, identifier, comment)+"/edit", url.Values{"content": {"second version"}}, 403)
	})

	t.Run("CommentOfAnotherGist", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "0")
		_, _, otherUsername, otherIdentifier := s.CreateGistAs(t, "bob", "0")

		s.Login(t, "alice")
		comment := postComment(t, s, username, identifier, "first version", 302)
		s.Request(t, "POST", commentPath(otherUsername, otherIdentifier, comment)+"/edit", url.Values{"content": {"second version"}}, 404)
		s.Request(t, "POST", "/"+username+"/"+identifier+"/comments/notanumber/edit", url.Values{"content": {"second version"}}, 404)
	})
}

func TestDeleteComment(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	s.Register(t, "thomas") // admin, as the first user
	s.Register(t, "alice")
	s.Register(t, "bob")

	t.Run("Author", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "0")

		s.Login(t, "alice")
		comment := postComment(t, s, username, identifier, "hello", 302)
		postComment(t, s, username, identifier, "hello again", 302)
		requireNbComments(t, username, identifier, 2)

		resp := s.Request(t, "POST", commentPath(username, identifier, comment)+"/delete", nil, 302)
		require.Equal(t, "/"+username+"/"+identifier+"#comments", resp.Header.Get("Location"))
		requireNbComments(t, username, identifier, 1)

		// already deleted
		s.Request(t, "POST", commentPath(username, identifier, comment)+"/delete", nil, 404)
		requireNbComments(t, username, identifier, 1)
	})

	t.Run("GistOwner", func(t *testing.T) {
		_, _, username, identifier := s.CreateGistAs(t, "bob", "0")

		s.Login(t, "alice")
		comment := postComment(t, s, username, identifier, "hello", 302)

		s.Login(t, "bob")
		s.Request(t, "POST", commentPath(username, identifier, comment)+"/delete", nil, 302)
		requireNbComments(t, username, identifier, 0)
	})

	t.Run("Admin", func(t *testing.T) {
		_, _, username, identifier := s.CreateGistAs(t, "bob", "0")

		s.Login(t, "alice")
		comment := postComment(t, s, username, identifier, "hello", 302)

		s.Login(t, "thomas")
		s.Request(t, "POST", commentPath(username, identifier, comment)+"/delete", nil, 302)
		requireNbComments(t, username, identifier, 0)
	})

	t.Run("OtherUser", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "0")

		s.Login(t, "alice")
		comment := postComment(t, s, username, identifier, "hello", 302)

		s.Login(t, "bob")
		s.Request(t, "POST", commentPath(username, identifier, comment)+"/delete", nil, 403)
		s.Logout()
		s.Request(t, "POST", commentPath(username, identifier, comment)+"/delete", nil, 302)
		requireNbComments(t, username, identifier, 1)
	})

	t.Run("ArchivedGist", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "0")

		s.Login(t, "alice")
		comment := postComment(t, s, username, identifier, "hello", 302)

		s.Login(t, "thomas")
		s.Request(t, "POST", "/"+username+"/"+identifier+"/archive", nil, 302)

		s.Login(t, "alice")
		s.Request(t, "POST", commentPath(username, identifier, comment)+"/delete", nil, 302)
		requireNbComments(t, username, identifier, 0)
	})
}

func TestCommentsCleanup(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	s.Register(t, "thomas")
	s.Register(t, "alice")
	s.Register(t, "bob")

	t.Run("DeleteAuthor", func(t *testing.T) {
		_, _, username, identifier := s.CreateGist(t, "0")
		_, _, otherUsername, otherIdentifier := s.CreateGistAs(t, "alice", "0")

		s.Login(t, "bob")
		postComment(t, s, username, identifier, "first", 302)
		postComment(t, s, username, identifier, "second", 302)
		postComment(t, s, otherUsername, otherIdentifier, "third", 302)
		s.Login(t, "alice")
		postComment(t, s, username, identifier, "fourth", 302)

		bob, err := db.GetUserByUsername("bob")
		require.NoError(t, err)
		require.NoError(t, bob.Delete())

		requireNbComments(t, username, identifier, 1)
		requireNbComments(t, otherUsername, otherIdentifier, 0)
	})

	t.Run("DeleteGist", func(t *testing.T) {
		_, gist, username, identifier := s.CreateGist(t, "0")

		s.Login(t, "alice")
		comment := postComment(t, s, username, identifier, "hello", 302)

		s.Login(t, "thomas")
		s.Request(t, "POST", "/"+username+"/"+identifier+"/delete", nil, 302)

		_, err := db.GetGistComment(gist.ID, comment.ID)
		require.Error(t, err)
	})
}
