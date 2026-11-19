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
The header must contain a username that users cannot change at the proxy. Accounts are matched by username, including the ones created with a password or OAuth, so a user who can rename themselves at the identity provider, or take the name of a deleted user, logs in to the account with that name, such as the one of an Opengist administrator. With Keycloak, for example, keep the "Edit username" realm option disabled.
:::

::: warning
The header is only trusted when the request comes directly from an address in `proxy-auth.trusted-cidrs`. This is the address of the TCP connection, not the `X-Forwarded-For` header. Any host in these ranges can log in as any user, so:
- keep `proxy-auth.trusted-cidrs` to the addresses of the proxies themselves, not a whole network such as `172.16.0.0/12` for Docker,
- make sure Opengist is not reachable without going through the proxy from any trusted address. Some container setups (e.g. rootless Docker or Podman) make every client appear to come from the same internal address: check which address Opengist sees, it is logged when the header is sent from an untrusted address,
- make sure the proxy always overwrites or removes the user header sent by the client, including on the routes it does not authenticate. With Nginx, a `location` without `auth_request` must still clear it, e.g. with `proxy_set_header X-Auth-Request-Preferred-Username "";`.
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

- The header value is the Opengist username (case-insensitive). If an account with this username already exists, including one created with a password or OAuth, the user is logged in as that account: when enabling proxy authentication on an existing instance, make sure existing usernames belong to the same people at the proxy.
- The header value must be printable ASCII; other values are refused, so that a lookalike name such as `admİn` cannot log in as `admin`.
- A request with the user header set more than once is refused, as the first value could come from the client; a repeated email header is ignored.
- Unknown users get an account created, unless signing up is disabled in the [admin panel](admin-panel.md). As with other sign-up methods, the first account created is an admin. The username must be a valid Opengist username (at most 24 letters, digits or dashes), which emails and most user IDs are not.
- Usernames belong to the proxy: users cannot change their username, and accounts cannot be created with the registration form, OAuth or LDAP. Otherwise, a user could take the username of someone who has not used Opengist yet, and keep access to their account.
- If LDAP is also enabled, LDAP credentials still log in to existing accounts without a password, including the accounts created by the proxy: LDAP and the proxy must give each username to the same person.
- The header takes precedence over the Opengist session. It is not stored in the session, so changing account at the proxy changes it in Opengist too. Logging out must be done at the proxy.
- Like the Opengist session cookie, the header is not used for requests sent by other sites, except to follow a link to Opengist: another site cannot, for instance, embed the private gists of its visitors with a `<script>` tag.
- Multi-factor authentication configured in Opengist is not asked for; it is up to the proxy.
- Removing a user at the proxy does not remove their Opengist account: the access tokens, SSH keys and password they set keep working for the API and Git. Delete the account from the admin panel as well.
- The [API](/docs/api.md) and Git over HTTP do not use the proxy header, they keep authenticating with access tokens and passwords. You may have to exclude them from the proxy authentication (e.g. with the oauth2-proxy `--skip-auth-route` option). With `--skip-auth-route`, the example above still clears the headers on these routes; a separate Nginx `location` must clear them itself.
- Proxy authentication is not available when Opengist listens on a Unix socket, which does not give the address of the proxy: Opengist refuses to start with both.
