# Install with Docker

Docker [images](https://github.com/thomiceli/opengist/pkgs/container/opengist) are available for each release :

```shell
docker pull ghcr.io/thomiceli/opengist:1
```

It can be used in a `docker-compose.yml` file :

1. Create a `docker-compose.yml` file with the following content
2. Run `docker compose up -d`
3. Opengist is now running on port 6157, you can browse http://localhost:6157

```yml
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
    environment:
      # OG_LOG_LEVEL: info
      # other configuration options
```

## Running as a non-root user

The container runs as a non-root user (`opengist`, UID/GID `1000`) — there is no
root entrypoint. Opengist stores its data in `/opengist`, so make sure that
directory is owned by `1000:1000` on the host:

```shell
mkdir -p ~/.opengist && sudo chown -R 1000:1000 ~/.opengist
```

If you would rather run as a different UID/GID (for example to match the owner of
an existing volume), set the `user` key instead, and own the data directory
accordingly:

```yml
services:
  opengist:
    # ...
    user: "1001:1001"
    volumes:
      - "./opengist-data:/opengist"
```

```shell
mkdir -p ./opengist-data && sudo chown -R 1001:1001 ./opengist-data
```

This also makes the image suitable for rootless Docker/Podman.
