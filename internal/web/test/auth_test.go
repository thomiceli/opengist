package test

import (
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"os"
	"os/exec"
	"path"
	"testing"
)

func TestRegister(t *testing.T) {
	s := Setup(t)
	defer Teardown(t, s)

	err := s.Request("GET", "/", nil, 302)
	require.NoError(t, err)

	err = s.Request("GET", "/register", nil, 200)
	require.NoError(t, err)

	user1 := db.UserDTO{Username: "thomas", Password: "thomas"}
	register(t, s, user1)

	user1db, err := db.GetUserById(1)
	require.NoError(t, err)
	require.Equal(t, user1.Username, user1db.Username)
	require.True(t, user1db.IsAdmin)

	err = s.Request("GET", "/", nil, 200)
	require.NoError(t, err)

	s.sessionCookie = ""

	user2 := db.UserDTO{Username: "thomas", Password: "azeaze"}
	err = s.Request("POST", "/register", user2, 200)
	require.Error(t, err)

	user3 := db.UserDTO{Username: "kaguya", Password: "kaguya"}
	register(t, s, user3)

	user3db, err := db.GetUserById(2)
	require.NoError(t, err)
	require.False(t, user3db.IsAdmin)

	s.sessionCookie = ""

	count, err := db.CountAll(db.User{})
	require.NoError(t, err)
	require.Equal(t, int64(2), count)
}

func TestLogin(t *testing.T) {
	s := Setup(t)
	defer Teardown(t, s)

	err := s.Request("GET", "/login", nil, 200)
	require.NoError(t, err)

	user1 := db.UserDTO{Username: "thomas", Password: "thomas"}
	register(t, s, user1)

	s.sessionCookie = ""

	login(t, s, user1)
	require.NotEmpty(t, s.sessionCookie)

	s.sessionCookie = ""

	user2 := db.UserDTO{Username: "thomas", Password: "azeaze"}
	user3 := db.UserDTO{Username: "azeaze", Password: ""}

	err = s.Request("POST", "/login", user2, 302)
	require.Empty(t, s.sessionCookie)
	require.Error(t, err)

	err = s.Request("POST", "/login", user3, 302)
	require.Empty(t, s.sessionCookie)
	require.Error(t, err)
}

func register(t *testing.T, s *TestServer, user db.UserDTO) {
	err := s.Request("POST", "/register", user, 302)
	require.NoError(t, err)
}

func login(t *testing.T, s *TestServer, user db.UserDTO) {
	err := s.Request("POST", "/login", user, 302)
	require.NoError(t, err)
}

func TestAnonymous(t *testing.T) {
	s := Setup(t)
	defer Teardown(t, s)

	user := db.UserDTO{Username: "thomas", Password: "azeaze"}
	register(t, s, user)

	err := s.Request("PUT", "/admin-panel/set-config", settingSet{"require-login", "1"}, 200)
	require.NoError(t, err)

	gist1 := db.GistDTO{
		Title:       "gist1",
		Description: "my first gist",
		VisibilityDTO: db.VisibilityDTO{
			Private: 0,
		},
		Name:    []string{"gist1.txt", "gist2.txt", "gist3.txt"},
		Content: []string{"yeah", "yeah\ncool", "yeah\ncool gist actually"},
		Topics:  "",
	}
	err = s.Request("POST", "/", gist1, 302)
	require.NoError(t, err)

	gist1db, err := db.GetGistByID("1")
	require.NoError(t, err)

	err = s.Request("GET", "/all", nil, 200)
	require.NoError(t, err)

	cookie := s.sessionCookie
	s.sessionCookie = ""

	err = s.Request("GET", "/all", nil, 302)
	require.NoError(t, err)

	// Should redirect to login if RequireLogin
	err = s.Request("GET", "/"+gist1db.User.Username+"/"+gist1db.Uuid, nil, 302)
	require.NoError(t, err)

	s.sessionCookie = cookie

	err = s.Request("PUT", "/admin-panel/set-config", settingSet{"allow-gists-without-login", "1"}, 200)
	require.NoError(t, err)

	s.sessionCookie = ""

	// Should return results
	err = s.Request("GET", "/"+gist1db.User.Username+"/"+gist1db.Uuid, nil, 200)
	require.NoError(t, err)

}

func TestGitOperations(t *testing.T) {
	s := Setup(t)
	defer Teardown(t, s)

	admin := db.UserDTO{Username: "thomas", Password: "thomas"}
	register(t, s, admin)
	s.sessionCookie = ""
	register(t, s, db.UserDTO{Username: "fujiwara", Password: "fujiwara"})
	s.sessionCookie = ""
	register(t, s, db.UserDTO{Username: "kaguya", Password: "kaguya"})

	gist1 := db.GistDTO{
		Title:       "kaguya-pub-gist",
		URL:         "kaguya-pub-gist",
		Description: "kaguya's first gist",
		VisibilityDTO: db.VisibilityDTO{
			Private: db.PublicVisibility,
		},
		Name: []string{"kaguya-file.txt"},
		Content: []string{
			"yeah",
		},
		Topics: "",
	}
	err := s.Request("POST", "/", gist1, 302)
	require.NoError(t, err)

	gist2 := db.GistDTO{
		Title:       "kaguya-unl-gist",
		URL:         "kaguya-unl-gist",
		Description: "kaguya's second gist",
		VisibilityDTO: db.VisibilityDTO{
			Private: db.UnlistedVisibility,
		},
		Name: []string{"kaguya-file.txt"},
		Content: []string{
			"cool",
		},
		Topics: "",
	}
	err = s.Request("POST", "/", gist2, 302)
	require.NoError(t, err)

	gist3 := db.GistDTO{
		Title:       "kaguya-priv-gist",
		URL:         "kaguya-priv-gist",
		Description: "kaguya's second gist",
		VisibilityDTO: db.VisibilityDTO{
			Private: db.PrivateVisibility,
		},
		Name: []string{"kaguya-file.txt"},
		Content: []string{
			"super",
		},
		Topics: "",
	}
	err = s.Request("POST", "/", gist3, 302)
	require.NoError(t, err)

	gitOperations := func(credentials, owner, url, filename string, expectErrorClone, expectErrorCheck, expectErrorPush bool) {
		log.Debug().Msgf("Testing %s %s %t %t %t", credentials, url, expectErrorClone, expectErrorCheck, expectErrorPush)
		err := clientGitClone(credentials, owner, url)
		if expectErrorClone {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
		err = clientCheckRepo(url, filename)
		if expectErrorCheck {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
		err = clientGitPush(url)
		if expectErrorPush {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
	}

	tests := []struct {
		credentials      string
		user             string
		url              string
		expectErrorClone bool
		expectErrorCheck bool
		expectErrorPush  bool
	}{
		{":", "kaguya", "kaguya-pub-gist", false, false, true},
		{":", "kaguya", "kaguya-unl-gist", false, false, true},
		{":", "kaguya", "kaguya-priv-gist", true, true, true},
		{"kaguya:kaguya", "kaguya", "kaguya-pub-gist", false, false, false},
		{"kaguya:kaguya", "kaguya", "kaguya-unl-gist", false, false, false},
		{"kaguya:kaguya", "kaguya", "kaguya-priv-gist", false, false, false},
		{"fujiwara:fujiwara", "kaguya", "kaguya-pub-gist", false, false, true},
		{"fujiwara:fujiwara", "kaguya", "kaguya-unl-gist", false, false, true},
		{"fujiwara:fujiwara", "kaguya", "kaguya-priv-gist", true, true, true},
	}

	for _, test := range tests {
		gitOperations(test.credentials, test.user, test.url, "kaguya-file.txt", test.expectErrorClone, test.expectErrorCheck, test.expectErrorPush)
	}

	login(t, s, admin)
	err = s.Request("PUT", "/admin-panel/set-config", settingSet{"require-login", "1"}, 200)
	require.NoError(t, err)

	testsRequireLogin := []struct {
		credentials      string
		user             string
		url              string
		expectErrorClone bool
		expectErrorCheck bool
		expectErrorPush  bool
	}{
		{":", "kaguya", "kaguya-pub-gist", true, true, true},
		{":", "kaguya", "kaguya-unl-gist", true, true, true},
		{":", "kaguya", "kaguya-priv-gist", true, true, true},
		{"kaguya:kaguya", "kaguya", "kaguya-pub-gist", false, false, false},
		{"kaguya:kaguya", "kaguya", "kaguya-unl-gist", false, false, false},
		{"kaguya:kaguya", "kaguya", "kaguya-priv-gist", false, false, false},
		{"fujiwara:fujiwara", "kaguya", "kaguya-pub-gist", false, false, true},
		{"fujiwara:fujiwara", "kaguya", "kaguya-unl-gist", false, false, true},
		{"fujiwara:fujiwara", "kaguya", "kaguya-priv-gist", true, true, true},
	}

	for _, test := range testsRequireLogin {
		gitOperations(test.credentials, test.user, test.url, "kaguya-file.txt", test.expectErrorClone, test.expectErrorCheck, test.expectErrorPush)
	}

	login(t, s, admin)
	err = s.Request("PUT", "/admin-panel/set-config", settingSet{"allow-gists-without-login", "1"}, 200)
	require.NoError(t, err)

	for _, test := range tests {
		gitOperations(test.credentials, test.user, test.url, "kaguya-file.txt", test.expectErrorClone, test.expectErrorCheck, test.expectErrorPush)
	}
}

func clientGitClone(creds string, user string, url string) error {
	return exec.Command("git", "clone", "http://"+creds+"@localhost:6157/"+user+"/"+url, path.Join(config.GetHomeDir(), "tmp", url)).Run()
}

func clientGitPush(url string) error {
	f, err := os.Create(path.Join(config.GetHomeDir(), "tmp", url, "newfile.txt"))
	if err != nil {
		return err
	}
	f.Close()

	_ = exec.Command("git", "-C", path.Join(config.GetHomeDir(), "tmp", url), "add", "newfile.txt").Run()
	_ = exec.Command("git", "-C", path.Join(config.GetHomeDir(), "tmp", url), "commit", "-m", "new file").Run()
	err = exec.Command("git", "-C", path.Join(config.GetHomeDir(), "tmp", url), "push", "origin", "master").Run()

	_ = os.RemoveAll(path.Join(config.GetHomeDir(), "tmp", url))

	return err
}

func clientCheckRepo(url string, file string) error {
	_, err := os.ReadFile(path.Join(config.GetHomeDir(), "tmp", url, file))
	return err
}
