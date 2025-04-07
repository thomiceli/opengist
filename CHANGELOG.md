# Changelog

## [1.10.0](https://github.com/thomiceli/opengist/compare/v1.9.1...v1.10.0) - 2025-04-07
See here how to [update](https://opengist.io/docs/update) Opengist.

### 🔴 Deprecations
_Removed in the next SemVer MAJOR version of Opengist._
* Use the configuration option `index`/`OG_INDEX` **instead of** `index.enabled`/`OG_INDEX_ENABLED`. The default value is `bleve`.
* The configuration `index.dirname`/`OG_INDEX_DIRNAME` will be removed. If you're using Bleve, the path of the index will be `opengist.index`.

### Added
- Helm Chart (#454)
- Meilisearch indexer (#444)
- Prometheus metrics (#439)
- Config to name the OIDC provider (#435)
- Read admin group from OIDC token claim (#445)
- More translation strings (#438)

### Fixed
- Garbled text display issues for non-English Unicode characters in browsers (#441)
- Test database when running `go test` (#442)
- Allow lag between admin invitation creation and test assertion (#452)
- gist.html using relative URL (#451)
- Do not hide file delete button on gist edit page (#447)

### Other
- Update deps Golang & JS deps (#455)

## [1.9.1](https://github.com/thomiceli/opengist/compare/v1.9.0...v1.9.1) - 2025-02-04
See here how to [update](https://opengist.io/docs/update) Opengist.

### Added
- More translation strings (#401)

### Fixed
- SQL query for MySQL/Postgres on user profile (#424)
- User avatar on gist likes list (#425)

### Other
- Update deps Golang & JS deps (#430)

## [1.9.0](https://github.com/thomiceli/opengist/compare/v1.8.4...v1.9.0) - 2025-02-02
See here how to [update](https://opengist.io/docs/update) Opengist.

### Added
- Topics (tags) for Gists (#413)
- Gist languages saved in database (#422)
- Search gists on user profile with title, visibility, language & topics (#422)
- Jdenticon for default avatars (#416)
- Git push option for description (#412)
- MIME type support for raw file serving (#417)

### Fixed
- Skip CSRF for embed gists (#402)
- Remove CSRF check for Git HTTP packs (#408)
- Replace path.Join with filepath.Join for file system paths (#414)
- Empty password error when trying to change the username (#418)
- Save content form on gist create error (#420)

### Other
- Refactor server code (#407)
- Remove memdb for gist init (#421)
- Added Opengist Docker images to Docker Hub

## [1.8.4](https://github.com/thomiceli/opengist/compare/v1.8.3...v1.8.4) - 2024-12-15
See here how to [update](/docs/update.md) Opengist.

### Added
- More translation strings (#398)
- Custom instance names (#399)

### Fixed
- Prevent passwords managers autofill on filename inputs (#357)

## [1.8.3](https://github.com/thomiceli/opengist/compare/v1.8.2...v1.8.3) - 2024-11-26
See here how to [update](/docs/update.md) Opengist.

### Changed
- Throw `warn` instead of `fatal` on Git global config init failure (#392)
- Define esbuild as a Javascript dependency for all other platforms (#393)

## [1.8.2](https://github.com/thomiceli/opengist/compare/v1.8.1...v1.8.2) - 2024-11-25
See here how to [update](/docs/update.md) Opengist.

### Added
- More translation strings (#373) (#388)

### Changed
- Enforce git config on startup (#383)
- Respect file scheme URIs for SQLite. (#387)

### Fixed
- Convert octal notation file names in Git (#380)
- Git clone on SSH with MySQL (#382)
- Escaping for embed gists (#381)

### Other
- Update deps Golang & JS deps

## [1.8.1](https://github.com/thomiceli/opengist/compare/v1.8.0...v1.8.1) - 2024-11-02
See here how to [update](/docs/update.md) Opengist.

### Changed
- Hide passkey login when login form is disabled (#369)

### Fixed
- Markdown preview (#368)
- confirm() popup messages (#370)

## [1.8.0](https://github.com/thomiceli/opengist/compare/v1.7.5...v1.8.0) - 2024-10-31
See here how to [update](https://opengist.io/docs/update) Opengist.

### 🔴 Deprecations 
_Removed in the next SemVer MAJOR version of Opengist._
* Use the configuration option `db-uri`/`OG_DB_URI` **instead of** `db-filename`/`OG_DB_FILENAME`.\
  More info [here](https://opengist.io/docs/configuration/databases/sqlite) if you plan to keep SQLite as a DBMS for Opengist.

### Added
- Postgres and MySQL databases support (#335)
- Passkeys & TOTP support + MFA (#341) (#342)
- Add/Remove admins (#337)
- Queriable shorter uuids (#338)
- Use Docker secrets (#340)
- SVG preview in Markdown (#346)
- Secret key definition & move the secret key file to its parent directory (#358)
- More translation strings (#339)

### Changed
- Separate OAuth unlink URL (#336)

### Fixed
- Adding multiple empty lines in editor. (#345)
- Config URL (#343)
- Send Markdown preview data as form params (#347)
- Fix oauth endpoint to support detecting https in 'Forwarded' header, enabling google support (#359)
- Use mail handle if OAuth nickname is empty (#362)

### Other
- Use go 1.23 and update deps (#354)
- Typos in README (#363)

## [1.7.5](https://github.com/thomiceli/opengist/compare/v1.7.4...v1.7.5) - 2024-09-12
See here how to [update](/docs/update.md) Opengist.

### Added
- New website for documentation using Vitepress [https://opengist.io](https://opengist.io) (#326)
- Ukrainian localization (#325)
- Dummy /metrics endpoint (#327)

## [1.7.4](https://github.com/thomiceli/opengist/compare/v1.7.3...v1.7.4) - 2024-09-09
See here how to [update](/docs/update.md) Opengist.

### Added
- More translations strings (#294) (#304)
- Hide change password form when login via password disabled (#314)
- File delete button on create editor (#320)
- Assets cache header 
- Hide secret values in admin config page
- Atomic pointer for indexer (#321)

### Fixed
- Fatal error using `cases.Title()` (#313)
- Search unlisted gist (#319)

### Other
- Removed logger `trace` and `fatal` levels (#322)

## [1.7.3](https://github.com/thomiceli/opengist/compare/v1.7.2...v1.7.3) - 2024-06-03
See here how to [update](/docs/update.md) Opengist.

### Added
- Setting to allow anonymous access to individual gists while still RequireLogin everywhere else (#229)
- Make edit visibility a toggle (#277)
- More translation strings (#274) (#281)
- String method to visibility (#276)

### Fixed
- Perms for http/ssh clone (#288)
- Fix translation string (#293)

### Other
- Update deps Golang & JS deps
- Check translations keys in CI (#279)
- Fix CI check for additional translations only (#289)

## [1.7.2](https://github.com/thomiceli/opengist/compare/v1.7.1...v1.7.2) - 2024-05-05
See here how to [update](/docs/update.md) Opengist.

### Added
- Docs: 
  - Run with systemd as a normal user (#254)
  - Kubernetes deployment (#258)
- More translation strings (#269) (#271)

### Changed
- Rework git log parsing and truncating (#260)
- Set Opengist version from git tags (#261)

### Fixed
- Missing preview button when editing .md gist (#259)
- Frontend (#267)
  - Fix mermaid display 
  - Move Login/Register buttons on mobile 
  - Set minimum width on avatar

### Other
- Use go 1.22 and update deps (#244)

## [1.7.1](https://github.com/thomiceli/opengist/compare/v1.7.0...v1.7.1) - 2024-04-05
See here how to [update](/docs/update.md) Opengist.

### Added
- Docs: More detailed variant for custom pages (#248)

### Fixed
- Auth page GitlabName Error (#242)
- Empty invitation on user creation (#247)

## [1.7.0](https://github.com/thomiceli/opengist/compare/v1.6.1...v1.7.0) - 2024-04-03
See here how to [update](/docs/update.md) Opengist.

Note: all sessions will be invalidated after this update.

### Added
- Custom logo configuration (#209)
- Custom static links (#234)
- Invitations for closed registrations (#233)
- Set gist visibility via Git push options (#215)
- Set gist URL and title via push options (#216)
- Specify custom names in the OAuth login buttons (#214)
- Markdown preview (#224)
- Reset a user password using CLI (#226)
- Translations (#207, #210)

### Changed
- Use filesystem session store (#240)
- Move Git hook logic to Opengist (#213)
- Increase login for 1 year (#222)

### Fixed
- Show theme change button on responsive devices (#225)
- New line literal in embed gists (#237)

### Other
- GitHub security updates
- New docker dev env (#220)

## [1.6.1](https://github.com/thomiceli/opengist/compare/v1.6.0...v1.6.1) - 2024-01-06
See here how to [update](/docs/update.md) Opengist.

### Added
- Healthcheck on Docker container (#204)
- Translations:
  - fr-FR (#201)

### Fixed
- Directory renaming on username change (#205)

## [1.6.0](https://github.com/thomiceli/opengist/compare/v1.5.3...v1.6.0) - 2024-01-04
See here how to [update](/docs/update.md) Opengist.

### Added
- Embedded gists (#179)
- Gist code search (#194)
- Custom URLS for gists (#183)
- Gist JSON data/metadata (#179)
- Keep default visibility when creating a gist on the UI (#155)
- Health check endpoint (#170)
- GitLab OAuth2 login (#174)
- Syntax highlighting for more file types (#176) 
- Checkable Markdown checkboxes (#182)
- Config:
  - Log output (#172) 
  - Default git branch name (#171)
- Change username setting (#190)
- Admin actions:
  - Synchronize all gists previews (#191)
  - Reset Git server hooks for all repositories (#191)
  - Index all gists (#194)
- Translations:
  - cs-CZ (#164)
  - zh-TW (#166, #195)
  - hu-HU (#185)
  - pt-BR (#193)
- Docs (#198)

### Changed
- Updated dependencies (#197):
  - Go `1.20` -> `1.21` 
  - JavaScript packages
  - NodeJS Docker image `18` -> `20`
  - Alpine Docker image `3.17` -> `3.19`

### Fixed
- Fix reverse proxy subpath support (#192)
- Fix undecoded gist content when going back to editing in the UI (#184)
- Fix outputting non-truncated large files for editon/zip download (#184)
- Allow dashes in usernames (#184)
- Delete SSH keys associated to deleted user (#184)
- Better error message when there is no files in gist (#184)
- Show if there is no files in gist preview (#184)
- Log parsing for the 11th empty commit (#184)
- Optimize reading gist files content (#186)

## [1.5.3](https://github.com/thomiceli/opengist/compare/v1.5.2...v1.5.3) - 2023-11-20
### Added
- es-ES translation (#139)
- Create/change account password (#156)
- Display OAuth error messages when HTTP 400 (#159)

### Fixed
- Git bare repository branch name creation (#157)
- Git file truncated output hanging (#157) 
- Home user directory detection handling (#145)
- UI changes (#158)

## [1.5.2](https://github.com/thomiceli/opengist/compare/v1.5.1...v1.5.2) - 2023-10-16
### Added
- zh-CN translation (#130)
- ru-RU translation (#135)
- config.yml usage in the Docker container (#131)
- Longer title and description (#129)

### Fixed
- Private gist visibility (#128)
- Dark background color in Markdown rendering (#137)
- Error handling for password hashes (#132)

## [1.5.1](https://github.com/thomiceli/opengist/compare/v1.5.0...v1.5.1) - 2023-09-29
### Added
- Hungarian translations (#123)

### Fixed
- .c and .h syntax highlighting (#119)
- Login page disabled depending on locale (#120)
- Syntax error on templates when calling locale function (#122)

## [1.5.0](https://github.com/thomiceli/opengist/compare/v1.4.2...v1.5.0) - 2023-09-26
### Added
- Private Gist visibility (#87)
- Create gists from a special Git HTTP server remote URL (#95)
- OIDC provider integration (#98)
- Translation system (#104)
- Run `git gc` on all repositories as admin (#90)
- Unit and integration tests (#97)
- Documentation (#110, #111)
- New logo (#103)

### Changed
- Use Non-CGO SQLite instead of CGO SQLite (#100)
- Various UI changes (#84, #93)
- Improved CI/CD pipeline (#99, #113)
- Improved git http semantics and repo obfuscation (#94)
- Updated Go deps (#102)

### Fixed
- Find command for Windows users (#89)
- Retain visibility when editing a gist (#83)
- Typo on admin index page (#85)
- ViteJS dev server (#91)
- Bugs (#105)

### Breaking changes
- Removed CONFIG env var
- Removed TLS server (#101)

## [1.4.2](https://github.com/thomiceli/opengist/compare/v1.4.1...v1.4.2) - 2023-07-17
### Added
- External url to HTML links & redirects (#75)
- Make unlisted gists not SEO crawlable (#78)
- Warning message on OAuth unlink (#79)

### Changed
- Redirect to `/all` when not logged in (#76)
- Removed Dev Docker image (#80)

## [1.4.1](https://github.com/thomiceli/opengist/compare/v1.4.0...v1.4.1) - 2023-06-25
### ⚠️ Docker users ⚠️
Opengist Docker volume has been changed from `/root/.opengist` to `/opengist`, do not forget to update your
`docker-compose.yml` file or any other Docker related configuration.

Please make a backup of your Opengist data directory before updating.

### Fixed
- Git message remote: `warning: unable to access '/root/.config/git/attributes': Permission denied` (#71)

## [1.4.0](https://github.com/thomiceli/opengist/compare/v1.3.0...v1.4.0) - 2023-06-23
### ⚠️ Docker users ⚠️
Opengist Docker volume has been changed from `/root/.opengist` to `/opengist`, do not forget to update your
`docker-compose.yml` file or any other Docker related configuration.

Please make a backup of your Opengist data directory before updating.

### Added
- Search gists, browse users snippets, likes and forks (#68)
- SQLite WAL journal mode by default (#54)
- Change SQLite journal mode via configuration (#54)
- Configuration via environment variables (#50)
- Docker dev image (#56)
- Choose Docker container/volumes owner via UID/GID (#63)

### Changed
- Docker volume changed from `/root/.opengist` to `/opengist` (#63)
- `DEV` environment variable renamed to `OG_DEV` (#64)
- Use `npx` in Makefile instead of `./node_modules/.bin` (#66)
- DEPRECATED: `OG_CONFIG` environment variable (#64)

### Fixed
- Gitea URL joins (#43, #61)
- Dark mode flickering (#44)
- Typos (#42)

## [1.3.0](https://github.com/thomiceli/opengist/compare/v1.2.0...v1.3.0) - 2023-05-27
### Added
- Disable login form via admin panel
- Syntax highlighting in Markdown code block (#29)
- Better UI for admin settings (#30)
- Disable Gravatar (#37)
- Swap between dark and light theme (#38)

### Changed
- Logs are now also appended to stdout
- Golang module name is now `github.com/thomiceli/opengist`

### Fixed
- First account registering with OAuth is now admin
- Fix HTML entities escaping in Markdown code block (#29)

## [1.2.0](https://github.com/thomiceli/opengist/compare/v1.1.1...v1.2.0) - 2023-05-01
### Added
- Restrict or unrestrict snippets visibility to anonymous users (#19)
- Go CI with Staticcheck

### Changed
- Filenames are now trimmed when creating a snippet (#20)
- SSH public key comments are now trimmed when adding a new key (#22)

### Fixed
- Respect ExternalUrl for OAuth (#21)
- SSH public key detection (#22)

## [1.1.1](https://github.com/thomiceli/opengist/compare/v1.1.0...v1.1.1) - 2023-04-20
### Fixed
- Git processes are now correctly killed

## [1.1.0](https://github.com/thomiceli/opengist/compare/v1.0.1...v1.1.0) - 2023-04-18
### Added
- GitHub and Gitea OAuth2 login
- Database migration system

### Changed
- Admin panel route from `/admin` route to `/admin-panel`
- Moved disable signup option to admin panel

### Fixed
- Truncate raw file (#4)
- Fix SSH key table constraints on user delete

## [1.0.1](https://github.com/thomiceli/opengist/compare/v1.0.0...v1.0.1) - 2023-04-12
### Changed
- Updated base footer
- Changed redirections when not logged in

## 1.0.0 - 2023-04-10
- Initial release
