# Init Gists via Git

Opengist allows you to create new snippets via Git over HTTP or SSH. You can create gists with either auto-generated URLs or custom URLs of your choice.

Simply init a new Git repository where your file(s) is/are located:

```shell
git init
git add .
git commit -m "My cool snippet"
```

### Option A: Regular URL

Create a gist with a custom URL using the format `http://opengist.url/username/custom-url`, where `username` is your authenticated username and `custom-url` is your desired gist identifier.

The gist must not exist yet if you want to create it, otherwise you will just push to the existing gist.

```shell
git remote add origin http://opengist.url/thomas/my-custom-gist
# or over SSH
git remote add origin ssh://opengist.url:2222/thomas/my-custom-gist.git

git push -u origin master
```

**Requirements for custom URLs:**
- The username must match your authenticated username
- URL format: `http://opengist.url/username/custom-url`, or `ssh://opengist.url:2222/username/custom-url` over SSH
- The custom URL becomes your gist's identifier and title
- `.git` suffix is automatically removed if present
- Over SSH, the custom URL may only contain alphanumeric characters and dashes, up to 32 characters

### Option B: Init endpoint

Use the special `http://opengist.url/init` endpoint to create a gist with an automatically generated URL:

```shell
git remote add origin http://opengist.url/init
# or over SSH
git remote add origin ssh://opengist.url:2222/init.git

git push -u origin master
```

::: tip
Over SSH, use the same address as the SSH clone URLs of your gists. With the instance's [OpenSSH server](/docs/administration/openssh), that's usually `gist@opengist.url:init.git`.
:::

## Authentication

Over HTTP, you'll be prompted for your username and password (or an [access token](./access-tokens) with read and write access to gists) when you push:

```shell
Username for 'http://opengist.url': thomas
Password for 'http://thomas@opengist.url': [your-password]
Enumerating objects: 3, done.
Counting objects: 100% (3/3), done.
Delta compression using up to 8 threads
Compressing objects: 100% (2/2), done.
Writing objects: 100% (3/3), 416 bytes | 416.00 KiB/s, done.
Total 3 (delta 0), reused 0 (delta 0), pack-reused 0
remote:
remote: Your new repository has been created here: http://opengist.url/thomas/6051e930f140429f9a2f3bb1fa101066
remote:
remote: If you want to keep working with your gist, you could set the remote URL via:
remote: git remote set-url origin http://opengist.url/thomas/6051e930f140429f9a2f3bb1fa101066
remote:
To http://opengist.url/init
 * [new branch]      master -> master
```

Over SSH, you are authenticated by your SSH key: add it to your account in **Settings → SSH keys** before pushing.

```shell
git push -u origin master
remote:
remote: Your new gist has been created here: http://opengist.url/thomas/3cf170a1c4264565af3f7b45aa383ae4
remote: If you want to keep working with your gist, you could set the Git remote URL via:
remote: git remote set-url origin ssh://opengist.url:2222/thomas/3cf170a1c4264565af3f7b45aa383ae4.git
remote:
To ssh://opengist.url:2222/init.git
 * [new branch]      master -> master
```

<video controls="controls" src="https://github.com/thomiceli/opengist/assets/27960254/3fe1a0ba-b638-4928-83a1-f38e46fea066" />