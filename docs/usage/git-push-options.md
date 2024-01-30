# Push Options

Opengist has support for a few [Git push options](https://git-scm.com/docs/git-push#Documentation/git-push.txt--oltoptiongt). 

These options are passed to `git push` command and can be used to change the metadata of a gist.

## Set URL

```shell
git push -o url=mygist # Will set the URL to https://opengist.example.com/user/mygist
```

## Change title

```shell
git push -o title=Gist123
git push -o title="My Gist 123"
```

## Change visibility

```shell
git push -o visibility=public
git push -o visibility=unlisted
git push -o visibility=private
```
