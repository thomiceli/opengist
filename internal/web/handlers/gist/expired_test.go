package gist_test

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/render"
	webtest "github.com/thomiceli/opengist/internal/web/test"
)

func expireGist(t *testing.T, gist *db.Gist) {
	gist.ExpiresAt = time.Now().Add(-time.Minute).Unix()
	require.NoError(t, gist.UpdateNoTimestamps())
}

// TestAllGistsExcludesExpired - expired gists not yet deleted by the background
// job are left out of the gist lists, the counts and filters shown with them
// and searches, for their owner too.
func TestAllGistsExcludesExpired(t *testing.T) {
	s := webtest.Setup(t)
	defer webtest.Teardown(t)

	s.Register(t, "thomas")
	s.Register(t, "alice")

	_, live, username, liveId := s.CreateGist(t, "0")

	// The expired gist is the only one with this topic and language.
	s.Login(t, "thomas")
	resp := s.Request(t, "POST", "/", url.Values{
		"title":   {"Expiring"},
		"name":    {"main.go"},
		"content": {"package main"},
		"topics":  {"expiring"},
		"private": {"0"},
	}, 302)
	parts := strings.Split(strings.TrimPrefix(resp.Header.Get("Location"), "/"), "/")
	require.Len(t, parts, 2)
	expired, err := db.GetGist(parts[0], parts[1])
	require.NoError(t, err)
	expiredId := parts[1]

	s.Request(t, "POST", "/"+username+"/"+liveId+"/like", nil, 302)
	s.Request(t, "POST", "/"+username+"/"+expiredId+"/like", nil, 302)
	s.Login(t, "alice")
	s.Request(t, "POST", "/"+username+"/"+liveId+"/fork", nil, 302)
	s.Logout()

	expireGist(t, expired)

	listedIds := func(t *testing.T, uri string) []uint {
		s.Request(t, "GET", uri, nil, 200)
		data, ok := s.ContextData("gists")
		require.True(t, ok)
		var ids []uint
		for _, g := range data.([]*render.RenderedGist) {
			ids = append(ids, g.ID)
		}
		return ids
	}

	for _, user := range []string{"", "thomas"} {
		t.Run("User="+user, func(t *testing.T) {
			s.Logout()
			if user != "" {
				s.Login(t, user)
			}

			for _, uri := range []string{
				"/-/all",
				"/-/liked",
				"/-/topics/expiring",
				"/" + username,
				"/" + username + "/-/liked",
			} {
				require.NotContains(t, listedIds(t, uri), expired.ID, uri)
			}
			require.Contains(t, listedIds(t, "/-/all"), live.ID)

			for _, uri := range []string{"/-/all", "/" + username, "/-/topics/expiring"} {
				s.Request(t, "GET", uri, nil, 200)
				languages, ok := s.ContextData("languages")
				require.True(t, ok)
				for _, l := range languages.([]struct {
					Language string
					Count    int64
				}) {
					require.NotEqual(t, "Go", l.Language, uri)
				}
			}

			s.Request(t, "GET", "/"+username, nil, 200)
			s.TestCtxData(t, map[string]any{
				"countFromUser": int64(1),
				"countLiked":    int64(1),
			})

			s.Request(t, "GET", "/-/topics", nil, 200)
			topics, ok := s.ContextData("topics")
			require.True(t, ok)
			for _, topic := range topics.([]*db.TopicCount) {
				require.NotEqual(t, "expiring", topic.Topic)
			}

			s.Request(t, "GET", "/-/users", nil, 200)
			users, ok := s.ContextData("users")
			require.True(t, ok)
			for _, u := range users.([]*db.UserWithNbGists) {
				if u.Username == username {
					require.Equal(t, int64(1), u.NbGists)
				}
			}

			s.Request(t, "GET", "/"+username+"/"+expiredId, nil, 404)
		})
	}

	t.Run("Search", func(t *testing.T) {
		gists, err := db.GetAllGistsFromSearch(0, "Expiring", 0, "created", "desc", "")
		require.NoError(t, err)
		require.Empty(t, gists)

		gists, err = db.GetAllGistsByIds([]uint{expired.ID, live.ID})
		require.NoError(t, err)
		require.Len(t, gists, 1)
		require.Equal(t, live.ID, gists[0].ID)
	})

	t.Run("Forked", func(t *testing.T) {
		_, forked, _, forkedId := s.CreateGist(t, "0")
		s.Login(t, "alice")
		s.Request(t, "POST", "/"+username+"/"+forkedId+"/fork", nil, 302)
		s.Logout()

		expireGist(t, forked)

		ids := listedIds(t, "/-/forked")
		require.NotContains(t, ids, forked.ID)
		require.Contains(t, ids, live.ID)
	})
}
