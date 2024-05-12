package auth

type AuthInfoProvider interface {
	RequireLogin() (bool, error)
	AllowGistsWithoutLogin() (bool, error)
}

func ShouldAllowUnauthenticatedGistAccess(prov AuthInfoProvider, isSingleGistAccess bool) (bool, error) {
	require, err := prov.RequireLogin()
	if err != nil {
		return false, err
	}
	allow, err := prov.AllowGistsWithoutLogin()
	if err != nil {
		return false, err
	}
	return !require || (isSingleGistAccess && allow), nil
}
