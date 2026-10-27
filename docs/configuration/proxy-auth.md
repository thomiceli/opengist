# Proxy authentication

Opengist can delegate authentication to a reverse proxy, such as [oauth2-proxy](https://oauth2-proxy.github.io/oauth2-proxy/), [Authelia](https://www.authelia.com) or [Authentik](https://goauthentik.io). The proxy authenticates the user, then forwards the request to Opengist with a header containing the username; Opengist logs in that user, creating the account on first login.

## Configuration

Set the header containing the username, and the addresses of the proxies allowed to set it, in the [configuration](cheat-sheet.md):

```yaml
proxy-auth.user-header: X-Auth-Request-Preferred-Username
# optional, used to set the email of new accounts
proxy-auth.email-header: X-Auth-Request-Email
# comma-separated list of CIDRs or IP addresses
proxy-auth.trusted-cidrs: 127.0.0.1,::1
```
```shell
OG_PROXY_AUTH_USER_HEADER=X-Auth-Request-Preferred-Username
OG_PROXY_AUTH_EMAIL_HEADER=X-Auth-Request-Email
OG_PROXY_AUTH_TRUSTED_CIDRS=127.0.0.1,::1
```

The header names depend on your proxy:

| Proxy                                        | Username header                     | Email header           |
|----------------------------------------------|-------------------------------------|------------------------|
| oauth2-proxy (`--set-xauthrequest`)          | `X-Auth-Request-Preferred-Username` | `X-Auth-Request-Email` |
| oauth2-proxy (`--pass-user-headers`)         | `X-Forwarded-Preferred-Username`    | `X-Forwarded-Email`    |
| Authelia                                     | `Remote-User`                       | `Remote-Email`         |
| Authentik                                    | `X-authentik-username`              | `X-authentik-email`    |

::: warning
The header is only trusted when the request comes directly from an address in `proxy-auth.trusted-cidrs`. This is the address of the TCP connection, not the `X-Forwarded-For` header. Still, make sure:
- Opengist is not reachable without going through the proxy from any trusted address,
- the proxy always overwrites or removes the user header sent by the client.
:::

## Example with Nginx and oauth2-proxy

Run oauth2-proxy with `--set-xauthrequest` (listening on `127.0.0.1:4180` here), then use the Nginx `auth_request` module to check each request against it:

```
server {
    listen 80;
    server_name opengist.example.com;

    location /oauth2/ {
        proxy_pass http://127.0.0.1:4180;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Auth-Request-Redirect $request_uri;
    }

    location = /oauth2/auth {
        proxy_pass http://127.0.0.1:4180;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-Uri $request_uri;
        proxy_set_header Content-Length "";
        proxy_pass_request_body off;
    }

    location / {
        auth_request /oauth2/auth;
        error_page 401 =403 /oauth2/sign_in;

        # always overwrite the headers sent by the client
        auth_request_set $user $upstream_http_x_auth_request_preferred_username;
        auth_request_set $email $upstream_http_x_auth_request_email;
        proxy_set_header X-Auth-Request-Preferred-Username $user;
        proxy_set_header X-Auth-Request-Email $email;

        proxy_pass http://127.0.0.1:6157;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

## Behavior

- The header value is the Opengist username (case-insensitive). If an account with this username already exists, including one created with a password or OAuth, the user is logged in as that account.
- Unknown users get an account created, unless signing up is disabled in the [admin panel](admin-panel.md). As with other sign-up methods, the first account created is an admin. The username must be a valid Opengist username (at most 24 letters, digits or dashes); prefer a "preferred username" header over a user ID or an email.
- The header takes precedence over the Opengist session. It is not stored in the session, so changing account at the proxy changes it in Opengist too. Logging out must be done at the proxy.
- Multi-factor authentication configured in Opengist is not asked for; it is up to the proxy.
- Renaming an account from the Opengist settings does not rename it at the proxy: the next request would log in, or create, the account with the old username.
- The [API](/docs/api.md) and Git over HTTP do not use the proxy header, they keep authenticating with access tokens and passwords. You may have to exclude them from the proxy authentication (e.g. with the oauth2-proxy `--skip-auth-route` option).
- Proxy authentication is not available when Opengist listens on a Unix socket.
