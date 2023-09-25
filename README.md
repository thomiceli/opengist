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

* Create public, unlisted or private snippets
* [Init](/docs/usage/init-via-git.md) / Clone / Pull / Push snippets **via Git** over HTTP or SSH
* Revisions history
* Syntax highlighting ; markdown & CSV support
* Like / Fork snippets
* Search for snippets ; browse users snippets, likes and forks
* Download raw files or as a ZIP archive
* OAuth2 login with GitHub, Gitea, and OpenID Connect
* Restrict or unrestrict snippets visibility to anonymous users
* Docker support
* [More...](/docs/index.md#features)

## Quick start

### With Docker

Docker [images](https://github.com/thomiceli/opengist/pkgs/container/opengist) are available for each release :

```shell
docker pull ghcr.io/thomiceli/opengist:1
```

It can be used in a `docker-compose.yml` file :

1. Create a `docker-compose.yml` file with the following content
2. Run `docker compose up -d`
3. Opengist is now running on port 6157, you can browse http://localhost:6157

```yml
version: "3"

services:
  opengist:
    image: ghcr.io/thomiceli/opengist:1.4
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

Requirements : [Git](https://git-scm.com/downloads) (2.20+), [Go](https://go.dev/doc/install) (1.20+), [Node.js](https://nodejs.org/en/download/) (16+)

```shell
git clone https://github.com/thomiceli/opengist
cd opengist
make
./opengist
```

Opengist is now running on port 6157, you can browse http://localhost:6157


## Documentation

The documentation is available in [/docs](/docs) directory.


## License

Opengist is licensed under the [AGPL-3.0 license](/LICENSE).
