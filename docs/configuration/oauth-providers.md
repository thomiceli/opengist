# Use OAuth providers

Opengist can be configured to use OAuth to authenticate users, with GitHub, Gitea, or OpenID Connect.

## GitHub

* Add a new OAuth app in your [GitHub account settings](https://github.com/settings/applications/new)
* Set 'Authorization callback URL' to `http://opengist.url/oauth/github/callback`
* Copy the 'Client ID' and 'Client Secret' and add them to the [configuration](cheat-sheet.md) :
  ```yaml
  github.client-key: <key>
  github.secret: <secret>
  ```
  ```shell
  OG_GITHUB_CLIENT_KEY=<key>
  OG_GITHUB_SECRET=<secret>
  ```


## GitLab

* Add a new OAuth app in Application settings from the [GitLab instance](https://gitlab.com/-/user_settings/applications)
* Set 'Redirect URI' to `http://opengist.url/oauth/gitlab/callback`
* Copy the 'Client ID' and 'Client Secret' and add them to the [configuration](cheat-sheet.md) :
  ```yaml
  gitlab.client-key: <key>
  gitlab.secret: <secret>
  # URL of the GitLab instance. Default: https://gitlab.com/
  gitlab.url: https://gitlab.com/
  ```
  ```shell
  OG_GITLAB_CLIENT_KEY=<key>
  OG_GITLAB_SECRET=<secret>
  # URL of the GitLab instance. Default: https://gitlab.com/
  OG_GITLAB_URL=https://gitlab.com/
  ```
  


## Gitea

* Add a new OAuth app in Application settings from the [Gitea instance](https://gitea.com/user/settings/applications)
* Set 'Redirect URI' to `http://opengist.url/oauth/gitea/callback`
* Copy the 'Client ID' and 'Client Secret' and add them to the [configuration](cheat-sheet.md) :
  ```yaml
  gitea.client-key: <key>
  gitea.secret: <secret>
  # URL of the Gitea instance. Default: https://gitea.com/
  gitea.url: http://localhost:3000
  ```
  ```shell
  OG_GITEA_CLIENT_KEY=<key>
  OG_GITEA_SECRET=<secret>
  # URL of the Gitea instance. Default: https://gitea.com/
  OG_GITEA_URL=http://localhost:3000
  ```
  


## OpenID Connect

* Add a new OAuth app in Application settings of your OIDC provider
* Set 'Redirect URI' to `http://opengist.url/oauth/openid-connect/callback`
* Copy the 'Client ID', 'Client Secret', and the discovery endpoint, and add them to the [configuration](cheat-sheet.md) :
  ```yaml
  oidc.provider-name: <provider-name>
  oidc.client-key: <key>
  oidc.secret: <secret>
  # Discovery endpoint of the OpenID provider. Generally something like http://auth.example.com/.well-known/openid-configuration
  oidc.discovery-url: http://auth.example.com/.well-known/openid-configuration
  ```
  ```shell
  OG_OIDC_PROVIDER_NAME=<provider-name>
  OG_OIDC_CLIENT_KEY=<key>
  OG_OIDC_SECRET=<secret>
  # Discovery endpoint of the OpenID provider. Generally something like http://auth.example.com/.well-known/openid-configuration
  OG_OIDC_DISCOVERY_URL=http://auth.example.com/.well-known/openid-configuration
  ```

### OIDC Admin Group

OpenGist supports automatic admin privilege assignment based on OIDC group claims. To configure this feature:
```yaml
oidc.group-claim-name: groups        # Name of the claim containing the groups
oidc.admin-group: admin-group-name   # Name of the group that should receive admin rights
```
```shell
OG_OIDC_GROUP_CLAIM_NAME=groups
OG_OIDC_ADMIN_GROUP=admin-group-name
```

The `group-claim-name` must match the name of the claim in your JWT token that contains the groups.

Users who are members of the configured `admin-group` will automatically receive admin privileges in OpenGist. These privileges are synchronized on every login.
