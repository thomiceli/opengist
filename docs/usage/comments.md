# Comments

Logged-in users can comment on any gist they can see. Comments are written in Markdown, rendered
like Markdown files, and limited to 3000 characters.

The last 5 comments are shown under the files of the gist. Every comment is listed, oldest first and
30 per page, on the Comments page of the gist, linked from its sidebar.

Comments are stored in the database, not in the Git repository of the gist, so they are not part of
its revisions and are not cloned along with it.

## Permissions

- The author of a comment can edit and delete it.
- The owner of a gist can delete any comment posted on it.
- Admins can delete any comment.

An archived gist is read-only: no new comment can be posted and existing ones can't be edited, but
they can still be deleted.

Comments are deleted along with their gist, or with their author's account.

## Disable comments

Comments are enabled by default. They can be disabled on the whole instance in the
[configuration](/docs/configuration/cheat-sheet), with `disable-comments: true` or
`OG_DISABLE_COMMENTS=true`. Existing comments are then hidden, not deleted, and appear again if
comments are enabled back.

## API

Comments can be listed, posted, edited and deleted with the
[REST API](/docs/api/listGistComments), under `/api/gists/{uuid}/comments`. Posting, editing and
deleting comments requires a token with the `gist:write` scope.
