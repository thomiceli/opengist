---
aside: true
---

# Opengist API Reference

Opengist exposes a REST API authenticated with Personal Access Tokens, intended
for programmatic access to gist and user resources.

The base URL for the API is 
```
https://opengist.example.com/api/
```

> OpenAPI 3.1 spec is available at
> [`openapi.yaml`](api/openapi.yaml).
>
> A running instance also serves the raw spec at `GET /api/openapi.yaml`.

## Getting an access token

The API authenticates with a Personal Access Token. To create one:

1. Go to **Settings**
2. Select the **Access Tokens** menu
3. Choose a name, select the scopes the token should grant and an optional
   expiration date, then click **Create Access Token**
4. Copy the token (starting with `og_`). It is shown only once.

Tokens carry per-resource scopes, each at read or read/write level:

| Scope | Grants |
|-------|--------|
| `gist:read` | Read gists, including the caller's private and unlisted ones |
| `gist:write` | Create, update, delete and fork gists |
| `user:read` | Read the authenticated user's account |
| `user:write` | Update the authenticated user and toggle likes |

## Authentication

Send the token in the `Authorization` header using the `Bearer` scheme:

```
Authorization: Bearer og_xxxxxxxx
```

Each endpoint documents the scope it requires in its **Headers** section.

Note that every endpoint requires authentication when an admin enables the "Require login" setting, which both works for the API and the web interface.

The single gist endpoints are available without authentication when an admin enables "Allow individual gists without login" setting.


## Schema

The API lives under the `/api/` prefix on the same host as your Opengist
instance. All data is sent and received as JSON.

Every endpoint responds with JSON unless specified otherwise
```
Content-Type: application/json
```

All timestamps are returned in ISO 8601 / RFC 3339 format, in UTC:
```
2024-01-01T00:00:00Z
```

## Pagination

List endpoints (such as `GET /gists`) return a JSON array and page the results.
Tune the window with these query parameters:

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `page` | integer | `1` | Page number, 1-based. |
| `per_page` | integer | `30` | Items per page (maximum `100`). |
| `since` | string (date-time) | — | Gist lists only: return only gists updated at or after this RFC 3339 timestamp. |

Pagination metadata is returned in the response headers:

| Header | Description |
|--------|-------------|
| `Link` | RFC 5988 links to other pages: `rel="next"` (when more pages exist) and `rel="prev"` (when `page > 1`). |
| `X-Page` | The current page number (1-based). |
| `X-Per-Page` | Items per page. |
| `X-Total` | Total number of items across all pages. |
| `X-Total-Pages` | Total number of pages, i.e. `ceil(total / per_page)`. |

The `Link` header is formatted as follows:

```
Link: <https://opengist.example/api/gists?page=2&per_page=30>; rel="next",
      <https://opengist.example/api/gists?page=1&per_page=30>; rel="prev"
```


## Disabling the API

The API is **enabled by default**. To disable it, define it in the config as follows

### YAML 
```yaml
api.enabled: false
```

### Environment variable
```shell
OG_API_ENABLED=false
```

While disabled, the routing layer returns `403` for every endpoint until it is
enabled again. Disabling does not revoke issued tokens — they resume working
once the API is turned back on.
