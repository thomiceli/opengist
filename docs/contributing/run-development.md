# Run Opengist in development mode

## With Docker

Assuming you have [Make](https://linux.die.net/man/1/make) installed,

```shell
# Clone the repository
git clone git@github.com:thomiceli/opengist.git
cd opengist

# Build the development image
make build_dev_docker
```

Now you can run the development image with the following command:

```shell
make run_dev_docker
```

Opengist is now running on port 6157, you can browse http://localhost:6157

## As a binary

Requirements:
* [Git](https://git-scm.com/downloads) (2.28+)
* [Go](https://go.dev/doc/install) (1.22+)
* [Node.js](https://nodejs.org/en/download/) (16+)
* [Make](https://linux.die.net/man/1/make) (optional, but easier)

```shell
git clone git@github.com:thomiceli/opengist.git
cd opengist
make watch
```

Opengist is now running on port 6157, you can browse http://localhost:6157
