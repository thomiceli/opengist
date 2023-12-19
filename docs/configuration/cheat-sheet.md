# Configuration Cheat Sheet

| YAML Config Key       | Environment Variable     | Default value         | Description                                                                                                                       |
|-----------------------|--------------------------|-----------------------|-----------------------------------------------------------------------------------------------------------------------------------|
| log-level             | OG_LOG_LEVEL             | `warn`                | Set the log level to one of the following: `trace`, `debug`, `info`, `warn`, `error`, `fatal`, `panic`.                           |
| external-url          | OG_EXTERNAL_URL          | none                  | Public URL for the Git HTTP/SSH connection. If not set, uses the URL from the request.                                            |
| opengist-home         | OG_OPENGIST_HOME         | home directory        | Path to the directory where Opengist stores its data.                                                                             |
| db-filename           | OG_DB_FILENAME           | `opengist.db`         | Name of the SQLite database file.                                                                                                 |
| sqlite.journal-mode   | OG_SQLITE_JOURNAL_MODE   | `WAL`                 | Set the journal mode for SQLite. More info [here](https://www.sqlite.org/pragma.html#pragma_journal_mode)                         |
| http.host             | OG_HTTP_HOST             | `0.0.0.0`             | The host on which the HTTP server should bind.                                                                                    |
| http.port             | OG_HTTP_PORT             | `6157`                | The port on which the HTTP server should listen.                                                                                  |
| http.git-enabled      | OG_HTTP_GIT_ENABLED      | `true`                | Enable or disable git operations (clone, pull, push) via HTTP. (`true` or `false`)                                                |
| ssh.git-enabled       | OG_SSH_GIT_ENABLED       | `true`                | Enable or disable git operations (clone, pull, push) via SSH. (`true` or `false`)                                                 |
| ssh.host              | OG_SSH_HOST              | `0.0.0.0`             | The host on which the SSH server should bind.                                                                                     |
| ssh.port              | OG_SSH_PORT              | `2222`                | The port on which the SSH server should listen.                                                                                   |
| ssh.external-domain   | OG_SSH_EXTERNAL_DOMAIN   | none                  | Public domain for the Git SSH connection, if it has to be different from the HTTP one. If not set, uses the URL from the request. |
| ssh.keygen-executable | OG_SSH_KEYGEN_EXECUTABLE | `ssh-keygen`          | Path to the SSH key generation executable.                                                                                        |
| github.client-key     | OG_GITHUB_CLIENT_KEY     | none                  | The client key for the GitHub OAuth application.                                                                                  |
| github.secret         | OG_GITHUB_SECRET         | none                  | The secret for the GitHub OAuth application.                                                                                      |
| gitlab.client-key     | OG_GITLAB_CLIENT_KEY     | none                  | The client key for the GitLab OAuth application.                                                                                  |
| gitlab.secret         | OG_GITLAB_SECRET         | none                  | The secret for the GitLab OAuth application.                                                                                      |
| gitlab.url            | OG_GITLAB_URL            | `https://gitlab.com/` | The URL of the GitLab instance.                                                                                                   |
| gitea.client-key      | OG_GITEA_CLIENT_KEY      | none                  | The client key for the Gitea OAuth application.                                                                                   |
| gitea.secret          | OG_GITEA_SECRET          | none                  | The secret for the Gitea OAuth application.                                                                                       |
| gitea.url             | OG_GITEA_URL             | `https://gitea.com/`  | The URL of the Gitea instance.                                                                                                    |
| oidc.client-key       | OG_OIDC_CLIENT_KEY       | none                  | The client key for the OpenID application.                                                                                        |
| oidc.secret           | OG_OIDC_SECRET           | none                  | The secret for the OpenID application.                                                                                            |
| oidc.discovery-url    | OG_OIDC_DISCOVERY_URL    | none                  | Discovery endpoint of the OpenID provider.                                                                                        |
