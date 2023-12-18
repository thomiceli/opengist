# Use OAuth providers

Opengist can be configured to use OAuth to authenticate users, with GitHub, Gitea, or OpenID Connect.

## Github

* Add a new OAuth app in your [Github account settings](https://github.com/settings/applications/new)
* Set 'Authorization callback URL' to `http://opengist.domain/oauth/github/callback`
* Copy the 'Client ID' and 'Client Secret' and add them to the [configuration](/docs/configuration/cheat-sheet.md) :
  ```yaml
  github.client-key: <key>
  github.secret: <secret>
  ```


## GitLab

* Add a new OAuth app in Application settings from the [GitLab instance](https://gitlab.com/-/user_settings/applications)
* Set 'Redirect URI' to `http://opengist.domain/oauth/gitlab/callback`
* Copy the 'Client ID' and 'Client Secret' and add them to the [configuration](/docs/configuration/cheat-sheet.md) :
  ```yaml
  gitlab.client-key: <key>
  gitlab.secret: <secret>
  # URL of the Gitlab instance. Default: https://gitlab.com/
  gitlab.url: https://gitlab.com/
  ```


## Gitea

* Add a new OAuth app in Application settings from the [Gitea instance](https://gitea.com/user/settings/applications)
* Set 'Redirect URI' to `http://opengist.domain/oauth/gitea/callback`
* Copy the 'Client ID' and 'Client Secret' and add them to the [configuration](/docs/configuration/cheat-sheet.md) :
  ```yaml
  gitea.client-key: <key>
  gitea.secret: <secret>
  # URL of the Gitea instance. Default: https://gitea.com/
  gitea.url: http://localhost:3000
  ```


## OpenID Connect

* Add a new OAuth app in Application settings of your OIDC provider
* Set 'Redirect URI' to `http://opengist.domain/oauth/openid-connect/callback`
* Copy the 'Client ID', 'Client Secret', and the discovery endpoint, and add them to the [configuration](/docs/configuration/cheat-sheet.md) :
  ```yaml
  oidc.client-key: <key>
  oidc.secret: <secret>
  # Discovery endpoint of the OpenID provider. Generally something like http://auth.example.com/.well-known/openid-configuration
  oidc.discovery-url: http://auth.example.com/.well-known/openid-configuration
  ```
