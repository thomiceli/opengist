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

## Rootless

By default the container starts as `root` and the entrypoint drops privileges to the
user defined by `UID`/`GID` (see above). 

If you'd rather have the container run as a
non-root user from the start — for example with `user:` in Compose, or under rootless
Docker/Podman — set the `user` key instead:

```yml
services:
  opengist:
    # ...
    user: "1001:1001"
    volumes:
      - "./opengist-data:/opengist"
```

In this mode the entrypoint runs Opengist directly as that user. 
Create the Opengist data directory and own it on the host first:
```shell
mkdir -p ./opengist-data && sudo chown -R 1001:1001 ./opengist-data
```

