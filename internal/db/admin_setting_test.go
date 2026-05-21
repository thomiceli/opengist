package db_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thomiceli/opengist/internal/db"
	webtest "github.com/thomiceli/opengist/internal/web/test"
)

func TestSettingApiEnabledDefault(t *testing.T) {
	_ = webtest.Setup(t)
	defer webtest.Teardown(t)

	v, err := db.GetSetting(db.SettingApiEnabled)
	require.NoError(t, err)
	require.Equal(t, "0", v, "api-enabled should default to '0' on fresh install")
}
