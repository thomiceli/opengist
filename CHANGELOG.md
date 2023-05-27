# Changelog

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
