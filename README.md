# Opengist

![GitHub release (latest SemVer)](https://img.shields.io/github/v/release/thomiceli/opengist?sort=semver)
![License](https://img.shields.io/github/license/thomiceli/opengist?color=blue)

A self-hosted pastebin **powered by Git**. [Try it here](https://opengist.thomice.li).

* [Features](#features)
* [Install](#install)
    * [With Docker](#with-docker)
    * [From source](#from-source)
* [Configuration](#configuration)
* [Administration](#administration)
    * [Use Nginx as a reverse proxy](#use-nginx-as-a-reverse-proxy)
    * [Use Fail2ban](#use-fail2ban)
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
* Avatars
* Responsive UI
* Enable or disable signups
* Admin panel : delete users/gists; clean database/filesystem by syncing gists
* SQLite database
* Logging
* Docker support

#### Todo

- [ ] Light mode
- [ ] Tests
- [ ] Search for snippets
- [ ] Embed snippets
- [ ] Filesystem/Redis support for user sessions
- [ ] Have a cool logo

## Install

### With Docker

A Docker [image](https://github.com/users/thomiceli/packages/container/package/opengist), available for each release, can be pulled

```
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
    image: ghcr.io/thomiceli/opengist:1
    container_name: opengist
    restart: unless-stopped
    ports:
      - "6157:6157" # HTTP port
      - "2222:2222" # SSH port, can be removed if you don't use SSH
    volumes:
      - "$HOME/.opengist:/root/.opengist"
    environment:
      CONFIG: |
        log-level: info
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

Opengist can be configured using YAML. The full configuration file is [config.yml](config.yml), each default key/value
pair can be overridden.

### With docker

Add a `CONFIG` environment variable in the `docker-compose.yml` file to the `opengist` service :

```diff
environment:
  CONFIG: |
    log-level: info
    ssh.git-enabled: false
    disable-signup: true
    # ...
```

### With binary

Create a `config.yml` file (you can reuse this [one](config.yml)) and run Opengist binary with the `--config` flag :

```shell
./opengist --config /path/to/config.yml
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
## License

Opengist is licensed under the [AGPL-3.0 license](LICENSE).
