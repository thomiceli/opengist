# Init Gists via Git

Opengist allows you to create new snippets via Git over HTTP. You can create gists with either auto-generated URLs or custom URLs of your choice.

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

git push -u origin master
```

**Requirements for custom URLs:**
- The username must match your authenticated username
- URL format: `http://opengist.url/username/custom-url`
- The custom URL becomes your gist's identifier and title
- `.git` suffix is automatically removed if present

### Option B: Init endpoint

Use the special `http://opengist.url/init` endpoint to create a gist with an automatically generated URL:

```shell
git remote add origin http://opengist.url/init

git push -u origin master
```

## Authentication

When you push, you'll be prompted to authenticate:

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

<video controls="controls" src="https://github.com/thomiceli/opengist/assets/27960254/3fe1a0ba-b638-4928-83a1-f38e46fea066" />