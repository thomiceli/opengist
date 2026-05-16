# REST API

Opengist exposes a REST API authenticated with Personal Access Tokens, intended for programmatic access to gist resources.

> **Authoritative OpenAPI 3.1 spec**: [`internal/web/handlers/api/openapi.yaml`](../../internal/web/handlers/api/openapi.yaml)
> **Live spec endpoint**: `GET /api/v1/openapi.yaml` on a running instance
>
> Import that URL into Postman, Insomnia, Bruno, Hoppscotch, or `openapi-generator` for an interactive UI or a generated client.

## Enabling the API

The API is **disabled by default**. An administrator must enable it explicitly:

1. Sign in as an administrator
2. Open **Admin Panel → Configuration**
3. Toggle **Enable REST API at /api/v1**

Disabling the API later does not revoke issued tokens; the routing layer simply returns `503` until it is enabled again.

## Creating a Personal Access Token

1. Sign in and open **Settings → Access Tokens**
2. Enter a name (e.g. `cli`) and choose scopes:
   - **Gist scope**: `Read` for read-only access; `Read+Write` to create, update or delete gists
   - **User scope**: `Read` is required to call `/api/v1/user`
3. Optionally set an expiry date
4. After submitting, the token is shown **once** — copy it immediately. Tokens look like `og_<64 hex>`.

## Authentication

Send the token in the `Authorization` header (Bearer recommended):

```
Authorization: Bearer og_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

The legacy `Authorization: Token og_xxx` form (used by the existing `.json` endpoints) is also accepted.

## Error responses

All errors share the same shape:

```json
{ "error": "human readable message", "code": "machine_readable_code" }
```

Common codes:

| HTTP | code | Meaning |
|------|------|---------|
| 401 | `unauthorized` | Missing / invalid / expired token |
| 403 | `forbidden` | Token lacks the required scope |
| 404 | `not_found` | Resource does not exist or is not visible to this token |
| 400 | `validation_failed` | Request body is invalid |
| 503 | `api_disabled` | Administrator has disabled the API |
| 500 | `internal_error` | Server-side failure |

When the API is disabled, the `503` response also includes a `hint` field pointing administrators to the toggle.

## Endpoints

All endpoints are prefixed with `/api/v1`.

### `GET /user` — current user

Requires the `user:read` scope.

```bash
curl -H "Authorization: Bearer og_xxx" https://opengist.example/api/v1/user
```

Response `200`:
```json
{
  "id": 1,
  "username": "alice",
  "email": "alice@example.com",
  "is_admin": false,
  "created_at": "2026-05-16T00:00:00Z"
}
```

### `GET /gists` — list gists

Requires the `gist:read` scope.

Query parameters:
- `page` (default `1`)
- `per_page` (default `10`, max `100`)
- `visibility` — `mine` (default; only gists owned by the current token's user) or `public` (site-wide public gists)

Response `200`:
```json
{
  "data": [
    {
      "uuid": "abc123",
      "title": "Hello",
      "description": "",
      "visibility": "public",
      "html_url": "/alice/abc123",
      "created_at": "2026-05-16T00:00:00Z",
      "updated_at": "2026-05-16T00:00:00Z",
      "owner": {"id": 1, "username": "alice"},
      "files": [{"filename": "a.txt", "size": 11}]
    }
  ],
  "page": 1,
  "per_page": 10,
  "total": 1
}
```

### `POST /gists` — create a gist

Requires the `gist:write` scope.

```bash
curl -X POST -H "Authorization: Bearer og_xxx" -H "Content-Type: application/json" \
  https://opengist.example/api/v1/gists \
  -d '{"title":"Hello","visibility":"public","files":[{"filename":"a.txt","content":"hello"}]}'
```

Response `201`: the full gist object including file contents.

### `GET /gists/{uuid}` — fetch a gist

Requires the `gist:read` scope. Private gists are only visible to their owner.

### `PATCH /gists/{uuid}` — update a gist

Requires the `gist:write` scope. The caller must be the owner. All body fields are optional:

```json
{
  "title": "New title",
  "description": "...",
  "visibility": "unlisted",
  "files": [{"filename": "a.txt", "content": "new content"}]
}
```

**`files` semantics**: providing the field **replaces all files**; omitting it leaves the existing files untouched.

### `DELETE /gists/{uuid}` — delete a gist

Requires the `gist:write` scope. Owner only. Returns `204 No Content`.

### `GET /gists/{uuid}/files/{filename}/raw` — raw file contents

Requires the `gist:read` scope. Returns `text/plain` with the raw bytes.

## Known limitations (v1)

- No rate limiting (do it at the reverse proxy if you need it)
- No binary file upload (string `content` only for now)
- No `user=`, `sort=`, or `order=` filters on the list endpoint
- No Like / Fork / Search / Revisions / Webhook endpoints
- No SSH key, invitation code, or admin operation endpoints
- For `visibility=public`, the `total` field is approximated by the current page size (the underlying query caps at 11 rows)
- File lists return at most 11 entries; `per_page>10` will not fetch more

These will be addressed in subsequent versions.

## End-to-end example

```bash
TOK=og_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
BASE=http://localhost:6157

# Current user
curl -s -H "Authorization: Bearer $TOK" $BASE/api/v1/user | jq

# Create a gist
curl -s -X POST -H "Authorization: Bearer $TOK" -H "Content-Type: application/json" \
  $BASE/api/v1/gists \
  -d '{"title":"E2E","visibility":"public","files":[{"filename":"hello.txt","content":"hi"}]}' \
  | jq

# List
curl -s -H "Authorization: Bearer $TOK" "$BASE/api/v1/gists?per_page=5" | jq

# Fetch one (replace UUID)
curl -s -H "Authorization: Bearer $TOK" $BASE/api/v1/gists/<UUID> | jq

# Raw file
curl -s -H "Authorization: Bearer $TOK" $BASE/api/v1/gists/<UUID>/files/hello.txt/raw

# Patch
curl -s -X PATCH -H "Authorization: Bearer $TOK" -H "Content-Type: application/json" \
  $BASE/api/v1/gists/<UUID> -d '{"title":"E2E renamed"}' | jq

# Delete
curl -s -X DELETE -H "Authorization: Bearer $TOK" -o /dev/null -w "%{http_code}\n" \
  $BASE/api/v1/gists/<UUID>
```
