# Opengist

![GitHub release (latest SemVer)](https://img.shields.io/github/v/release/thomiceli/opengist?sort=semver)
![License](https://img.shields.io/github/license/thomiceli/opengist?color=blue)
![GitHub Workflow Status](https://img.shields.io/github/actions/workflow/status/thomiceli/opengist/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/thomiceli/opengist)](https://goreportcard.com/report/github.com/thomiceli/opengist)

A self-hosted pastebin **powered by Git**. [Try it here](https://opengist.thomice.li).

* [Features](#features)
* [Install](#install)
    * [With Docker](#with-docker)
    * [From source](#from-source)
* [Configuration](#configuration)
    * [Via YAML file](#configuration-via-yaml-file)
    * [Via Environment Variables](#configuration-via-environment-variables)
* [Administration](#administration)
    * [Use Nginx as a reverse proxy](#use-nginx-as-a-reverse-proxy)
    * [Use Fail2ban](#use-fail2ban)
* [Configure OAuth](#configure-oauth)
* [License](#license)

## Features

* Create public or unlisted snippets
* Clone / Pull / Push snippets **via Git** over HTTP or SSH
* Revisions history
* Syntax highlighting ; markdown & CSV support
* Like / Fork snippets
* Search for all snippets or for certain users snippets
* Editor with indentation mode & size ; drag and drop files
* Download raw files or as a ZIP archive
* OAuth2 login with GitHub and Gitea
* Avatars via Gravatar or OAuth2 providers
* Light/Dark mode
* Responsive UI
* Enable or disable signups
* Restrict or unrestrict snippets visibility to anonymous users
* Admin panel : delete users/gists; clean database/filesystem by syncing gists
* SQLite database
* Logging
* Docker support

#### Todo

- [ ] Tests
- [ ] Search for snippets
- [ ] Embed snippets
- [ ] Filesystem/Redis support for user sessions
- [ ] Have a cool logo

## Install

### With Docker

A Docker [image](https://github.com/thomiceli/opengist/pkgs/container/opengist), available for each release, can be pulled

```shell
docker pull ghcr.io/thomiceli/opengist       # most recent release

docker pull ghcr.io/thomiceli/opengist:dev   # latest development version
```

It can be used in a `docker-compose.yml` file :

1. Create a `docker-compose.yml` file with the following content
2. Run `docker compose up -d`
3. Opengist is now running on port 6157, you can browse http://localhost:6157

```yml
version: "3"

services:
  opengist:
    image: ghcr.io/thomiceli/opengist:1.3
    container_name: opengist
    restart: unless-stopped
    ports:
      - "6157:6157" # HTTP port
      - "2222:2222" # SSH port, can be removed if you don't use SSH
    volumes:
      - "$HOME/.opengist:/opengist"
```

You can define which user/group should run the container and own the files by setting the `UID` and `GID` environment variables :

```yml
services:
  opengist:
    # ...
    environment:
      UID: 1001
      GID: 1001
```

### From source

Requirements : [Git](https://git-scm.com/downloads) (2.20+), [Go](https://go.dev/doc/install) (1.19+), [Node.js](https://nodejs.org/en/download/) (16+)

```shell
git clone https://github.com/thomiceli/opengist
cd opengist
make
./opengist
```

Opengist is now running on port 6157, you can browse http://localhost:6157

## Configuration

Opengist provides flexible configuration options through either a YAML file and/or environment variables. 
You would only need to specify the configuration options you want to change — for any config option left untouched, Opengist will simply apply the default values.

<details>
<summary>Configuration option list</summary>

| YAML Config Key       | Environment Variable     | Default value        | Description                                                                                                                       | 
|-----------------------|--------------------------|----------------------|-----------------------------------------------------------------------------------------------------------------------------------|
| log-level             | OG_LOG_LEVEL             | `warn`               | Set the log level to one of the following: `trace`, `debug`, `info`, `warn`, `error`, `fatal`, `panic`.                           |
| external-url          | OG_EXTERNAL_URL          | none                 | Public URL for the Git HTTP/SSH connection. If not set, uses the URL from the request.                                            |
| opengist-home         | OG_OPENGIST_HOME         | home directory       | Path to the directory where Opengist stores its data.                                                                             |
| db-filename           | OG_DB_FILENAME           | `opengist.db`        | Name of the SQLite database file.                                                                                                 |
| sqlite.journal-mode   | OG_SQLITE_JOURNAL_MODE   | `WAL`                | Set the journal mode for SQLite. More info [here](https://www.sqlite.org/pragma.html#pragma_journal_mode)                         |
| http.host             | OG_HTTP_HOST             | `0.0.0.0`            | The host on which the HTTP server should bind.                                                                                    |
| http.port             | OG_HTTP_PORT             | `6157`               | The port on which the HTTP server should listen.                                                                                  |
| http.git-enabled      | OG_HTTP_GIT_ENABLED      | `true`               | Enable or disable git operations (clone, pull, push) via HTTP. (`true` or `false`)                                                |
| http.tls-enabled      | OG_HTTP_TLS_ENABLED      | `false`              | Enable or disable TLS for the HTTP server. (`true` or `false`)                                                                    |
| http.cert-file        | OG_HTTP_CERT_FILE        | none                 | Path to the TLS certificate file if TLS is enabled.                                                                               |
| http.key-file         | OG_HTTP_KEY_FILE         | none                 | Path to the TLS key file if TLS is enabled.                                                                                       |
| ssh.git-enabled       | OG_SSH_GIT_ENABLED       | `true`               | Enable or disable git operations (clone, pull, push) via SSH. (`true` or `false`)                                                 |
| ssh.host              | OG_SSH_HOST              | `0.0.0.0`            | The host on which the SSH server should bind.                                                                                     |
| ssh.port              | OG_SSH_PORT              | `2222`               | The port on which the SSH server should listen.                                                                                   |
| ssh.external-domain   | OG_SSH_EXTERNAL_DOMAIN   | none                 | Public domain for the Git SSH connection, if it has to be different from the HTTP one. If not set, uses the URL from the request. |
| ssh.keygen-executable | OG_SSH_KEYGEN_EXECUTABLE | `ssh-keygen`         | Path to the SSH key generation executable.                                                                                        |
| github.client-key     | OG_GITHUB_CLIENT_KEY     | none                 | The client key for the GitHub OAuth application.                                                                                  |
| github.secret         | OG_GITHUB_SECRET         | none                 | The secret for the GitHub OAuth application.                                                                                      |
| gitea.client-key      | OG_GITEA_CLIENT_KEY      | none                 | The client key for the Gitea OAuth application.                                                                                   |
| gitea.secret          | OG_GITEA_SECRET          | none                 | The secret for the Gitea OAuth application.                                                                                       |
| gitea.url             | OG_GITEA_URL             | `https://gitea.com/` | The URL of the Gitea instance.                                                                                                    |

</details>

### Configuration via YAML file

The configuration file must be specified when launching the application, using the `--config` flag followed by the path to your YAML file.

```shell
./opengist --config /path/to/config.yml
```

You can start by copying and/or modifying the provided [config.yml](config.yml) file.

### Configuration via Environment Variables

Usage with Docker Compose :

```yml
services:
  opengist:
    # ...
    environment:
      OG_LOG_LEVEL: "info"
      # etc.
```
Usage via command line :

```shell
OG_LOG_LEVEL=info ./opengist
```

## Administration

### Use Nginx as a reverse proxy

Configure Nginx to proxy requests to Opengist. Here is an example configuration file :
```
server {
    listen 80;
    server_name opengist.example.com;

    location / {
        proxy_pass http://127.0.0.1:6157;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

Then run :
```shell
service nginx restart
```

### Use Fail2ban

Fail2ban can be used to ban IPs that try to bruteforce the login page.
Log level must be set at least to `warn`.

Add this filter in `etc/fail2ban/filter.d/opengist.conf` :
```ini
[Definition]
failregex =  Invalid .* authentication attempt from <HOST>
ignoreregex =
```

Add this jail in `etc/fail2ban/jail.d/opengist.conf` :
```ini
[opengist]
enabled = true
filter = opengist
logpath = /home/*/.opengist/log/opengist.log
maxretry = 10
findtime = 3600
bantime = 600
banaction = iptables-allports
port = anyport
```

Then run
```shell
service fail2ban restart
```

## Configure OAuth

Opengist can be configured to use OAuth to authenticate users, with GitHub or Gitea.

<details>
<summary>Integrate Github</summary>

* Add a new OAuth app in your [Github account settings](https://github.com/settings/applications/new)
* Set 'Authorization callback URL' to `http://opengist.domain/oauth/github/callback`
* Copy the 'Client ID' and 'Client Secret' and add them to the configuration :
  ```yaml
  github.client-key: <key>
  github.secret: <secret>
  ```
</details>

<details>
<summary>Integrate Gitea</summary>

* Add a new OAuth app in Application settings from the [Gitea instance](https://gitea.com/user/settings/applications)
* Set 'Redirect URI' to `http://opengist.domain/oauth/gitea/callback`
* Copy the 'Client ID' and 'Client Secret' and add them to the configuration :
  ```yaml
  gitea.client-key: <key>
  gitea.secret: <secret>
  # URL of the Gitea instance. Default: https://gitea.com/
  gitea.url: http://localhost:3000
  ```
</details>

## License

Opengist is licensed under the [AGPL-3.0 license](LICENSE).
