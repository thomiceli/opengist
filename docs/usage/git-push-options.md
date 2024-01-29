# Push Options

Opengist has support for a few [Git push options](https://git-scm.com/docs/git-push#Documentation/git-push.txt--oltoptiongt). 

These options are passed to `git push` command and can be used to change the metadata of a gist.

## Change visibility

```shell
git push -o visibility=public
git push -o visibility=unlisted
git push -o visibility=private
```
