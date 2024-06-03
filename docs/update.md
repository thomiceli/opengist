# Update

## Make a backup

Before updating, always make sure to backup the Opengist home directory, where all the data is stored. 

You can do so by copying the `~/.opengist` directory (default location).

```shell
cp -r ~/.opengist ~/.opengist.bak
```

## Install the new version

### With Docker

Pull the last version of Opengist
```shell
docker pull ghcr.io/thomiceli/opengist:1
```

And restart the container, using `docker compose up -d` for example if you use docker compose.

### Via binary

Stop the running instance; then like your first installation of Opengist, download the archive for your system from the release page [here](https://github.com/thomiceli/opengist/releases/latest), and extract it.

```shell
# example for linux amd64
wget https://github.com/thomiceli/opengist/releases/download/v1.7.3/opengist1.7.3-linux-amd64.tar.gz

tar xzvf opengist1.7.3-linux-amd64.tar.gz
cd opengist
chmod +x opengist
./opengist # with or without `--config config.yml`
```

### From source

Stop the running instance; then pull the last changes from the master branch, and build the new version.

```shell
git pull
make
./opengist
```

## Restore the backup

If you have any issue with the new version, you can restore the backup you made before updating.

```shell
rm -rf ~/.opengist
cp -r ~/.opengist.bak ~/.opengist
```

Then run the old version of Opengist again.
