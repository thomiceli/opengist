# Opengist

<img height="108px" src="https://raw.githubusercontent.com/thomiceli/opengist/master/public/opengist.svg" alt="Opengist" align="right" />

Opengist е **self-hosted** Pastebin, **базиран на Git**. Всички снипети се съхраняват в Git хранилище и могат да бъдат
четени и/или променяни чрез стандартни Git команди или през уеб интерфейса.
Подобен е на [GitHub Gist](https://gist.github.com/), но е с отворен код и може да бъде self-hosted.

[Home Page](https://opengist.io) • [Documentation](https://opengist.io/docs) • [Discord](https://discord.gg/9Pm3X5scZT) • [Demo](https://demo.opengist.io)


![GitHub release (latest SemVer)](https://img.shields.io/github/v/release/thomiceli/opengist?sort=semver)
![License](https://img.shields.io/github/license/thomiceli/opengist?color=blue)
[![Go CI](https://github.com/thomiceli/opengist/actions/workflows/go.yml/badge.svg)](https://github.com/thomiceli/opengist/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/thomiceli/opengist)](https://goreportcard.com/report/github.com/thomiceli/opengist)
[![Translate](https://tr.opengist.io/widget/_/svg-badge.svg)](https://tr.opengist.io/projects/_/opengist/)

## Features

* Създаване на публични, скрити (unlisted) или частни снипети
* [Init](/docs/usage/init-via-git.md) / Clone / Pull / Push на снипети **чрез Git** по HTTP или SSH
* Syntax highlighting; поддръжка на markdown и CSV
* Търсене в кода на снипети; разглеждане на снипети на потребители, харесвания и fork-ове
* Добавяне на теми (topics) към снипети
* Вграждане (embed) на снипети в други уебсайтове
* История на ревизиите
* Харесване (Like) / Fork на снипети
* Изтегляне на raw файлове или като ZIP архив
* OAuth2 вход с GitHub, GitLab, Gitea и OpenID Connect
* Ограничаване или премахване на ограниченията за видимост на снипети за анонимни потребители
* Docker поддръжка / Helm Chart
* [More...](/docs/introduction.md#features)

## Quick start

### With Docker

Docker [images](https://github.com/thomiceli/opengist/pkgs/container/opengist) са налични за всяко издание:

```shell
docker pull ghcr.io/thomiceli/opengist:1.11
```

Може да се използва в `docker-compose.yml` файл:

1. Създайте `docker-compose.yml` файл със следното съдържание
2. Стартирайте `docker compose up -d`
3. Opengist вече работи на порт 6157, можете да го отворите на http://localhost:6157

```yml
services:
  opengist:
    image: ghcr.io/thomiceli/opengist:1.11
    container_name: opengist
    restart: unless-stopped
    ports:
      - "6157:6157" # HTTP порт
      - "2222:2222" # SSH порт, може да бъде премахнат, ако не използвате SSH
    volumes:
      - "$HOME/.opengist:/opengist"
```

Можете да определите кой потребител/група да стартира контейнера и да притежава файловете, като зададете променливите на средата `UID` и `GID`:

```yml
services:
  opengist:
    # ...
    environment:
      UID: 1001
      GID: 1001
```

### Via binary

Изтеглете архива за вашата система от страницата с изданията [тук](https://github.com/thomiceli/opengist/releases/latest) и го извлечете.

```shell
# example for linux amd64
wget https://github.com/thomiceli/opengist/releases/download/v1.11.1/opengist1.11.1-linux-amd64.tar.gz

tar xzvf opengist1.11.1-linux-amd64.tar.gz
cd opengist
chmod +x opengist
./opengist # with or without `--config config.yml`
```

Opengist вече работи на порт 6157, можете да го отворите на http://localhost:6157

### From source

Изисквания: [Git](https://git-scm.com/downloads) (2.28+), [Go](https://go.dev/doc/install) (1.23+), [Node.js](https://nodejs.org/en/download/) (16+), [Make](https://linux.die.net/man/1/make) (по избор, но улеснява процеса)

```shell
git clone https://github.com/thomiceli/opengist
cd opengist
make
./opengist
```

Opengist вече работи на порт 6157, можете да го отворите на http://localhost:6157

---

За създаване и стартиране на development среда вижте [run-development.md](/docs/contributing/development.md).

## Documentation

Документацията е налична на [https://opengist.io/](https://opengist.io/) или в директорията [/docs](/docs).

## License

Opengist е лицензиран под [AGPL-3.0 license](/LICENSE).
