# Init Gists via Git

Opengist allows you to create new snippets via Git over HTTP.

Simply init a new Git repository where your file(s) is/are located:

```shell
git init
git add .
git commit -m "My cool snippet"
```

Then add this Opengist special remote URL and push your changes:

```shell
git remote add origin http://localhost:6157/init

git push -u origin master
```

Log in with your Opengist account credentials, and your snippet will be created at the specified URL:

```shell
Username for 'http://localhost:6157': thomas
Password for 'http://thomas@localhost:6157':
Enumerating objects: 3, done.
Counting objects: 100% (3/3), done.
Delta compression using up to 8 threads
Compressing objects: 100% (2/2), done.
Writing objects: 100% (3/3), 416 bytes | 416.00 KiB/s, done.
Total 3 (delta 0), reused 0 (delta 0), pack-reused 0
remote:
remote: Your new repository has been created here: http://localhost:6157/thomas/6051e930f140429f9a2f3bb1fa101066
remote:
remote: If you want to keep working with your gist, you could set the remote URL via:
remote: git remote set-url origin http://localhost:6157/thomas/6051e930f140429f9a2f3bb1fa101066
remote:
To http://localhost:6157/init
 * [new branch]      master -> master
```

<video controls="controls" src="https://github.com/thomiceli/opengist/assets/27960254/3fe1a0ba-b638-4928-83a1-f38e46fea066" />