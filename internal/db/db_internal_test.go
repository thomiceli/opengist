package db

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseDBURISocket(t *testing.T) {
	t.Run("MySQL unix socket", func(t *testing.T) {
		info, err := parseDBURI("mysql://root:passwd@/opengist_db?socket=/var/run/mysqld/mysqld.sock")
		require.NoError(t, err)
		require.Equal(t, MySQL, info.Type)
		require.Equal(t, "/var/run/mysqld/mysqld.sock", info.Socket)
		require.Equal(t, "root", info.User)
		require.Equal(t, "passwd", info.Password)
		require.Equal(t, "opengist_db", info.Database)
		require.Empty(t, info.Host)
		require.Empty(t, info.Port)
	})

	t.Run("PostgreSQL unix socket", func(t *testing.T) {
		info, err := parseDBURI("postgres://postgres:passwd@/opengist_db?socket=/var/run/postgresql")
		require.NoError(t, err)
		require.Equal(t, PostgreSQL, info.Type)
		require.Equal(t, "/var/run/postgresql", info.Socket)
		require.Equal(t, "postgres", info.User)
		require.Equal(t, "passwd", info.Password)
		require.Equal(t, "opengist_db", info.Database)
	})

	t.Run("PostgreSQL socket keeps sslmode", func(t *testing.T) {
		info, err := parseDBURI("postgres://postgres:passwd@/opengist_db?socket=/var/run/postgresql&sslmode=require")
		require.NoError(t, err)
		require.Equal(t, "/var/run/postgresql", info.Socket)
		require.Equal(t, "require", info.SSLMode)
	})

	t.Run("TCP connection has no socket", func(t *testing.T) {
		info, err := parseDBURI("mysql://root:passwd@localhost:3306/opengist_db")
		require.NoError(t, err)
		require.Empty(t, info.Socket)
		require.Equal(t, "localhost", info.Host)
		require.Equal(t, "3306", info.Port)
	})
}
