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

## Change description

```shell
git push -o description="This is my gist description"
```

## Change visibility

```shell
git push -o visibility=public
git push -o visibility=unlisted
git push -o visibility=private
```

## Change topics

```shell
git push -o topics="golang devops"
```

## Set expiration

Only applies when creating a gist. The value is either a preset
(`1hour`, `12hours`, `1day`, `7days`, `15days`) or a custom date
(RFC3339, e.g. `2026-01-02T15:04:05Z`).

```shell
git push -o expire=1day 
git push -o expire=2026-01-02T15:04:05Z
```
