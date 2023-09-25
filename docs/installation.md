# Installation

## With Docker

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
    image: ghcr.io/thomiceli/opengist:1
    container_name: opengist
    restart: unless-stopped
    ports:
      - "6157:6157" # HTTP port
      - "2222:2222" # SSH port, can be removed if you don't use SSH
    volumes:
      - "$HOME/.opengist:/opengist"
```

You can define which user/group should run the container and own the files by setting the `UID` and `GID` environment 
variables :

```yml
services:
  opengist:
    # ...
    environment:
      UID: 1001
      GID: 1001
```


## From source

Requirements : 
* [Git](https://git-scm.com/downloads) (2.20+)
* [Go](https://go.dev/doc/install) (1.20+)
* [Node.js](https://nodejs.org/en/download/) (16+)

```shell
git clone https://github.com/thomiceli/opengist
cd opengist
make
./opengist
```

Opengist is now running on port 6157, you can browse http://localhost:6157
