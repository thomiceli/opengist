# Opengist

<img height="108px" src="https://raw.githubusercontent.com/thomiceli/opengist/master/public/opengist.svg" alt="Opengist" align="right" />

Opengist is a **self-hosted** pastebin **powered by Git**. All snippets are stored in a Git repository and can be
read and/or modified using standard Git commands, or with the web interface. 
It is similiar to [GitHub Gist](https://gist.github.com/), but open-source and could be self-hosted.

Written in [Go](https://go.dev), Opengist aims to be fast and easy to deploy.


## Features

* Create public, unlisted or private snippets
* [Init](usage/init-via-git.md) / Clone / Pull / Push snippets **via Git** over HTTP or SSH
* Syntax highlighting ; markdown & CSV support
* Search code in snippets ; browse users snippets, likes and forks
* Embed snippets in other websites
* Revisions history
* Like / Fork snippets
* Editor with indentation mode & size ; drag and drop files
* Download raw files or as a ZIP archive
* Retrieve snippet data/metadata via a JSON API
* OAuth2 login with GitHub, GitLab, Gitea, and OpenID Connect
* Avatars via Gravatar or OAuth2 providers
* Light/Dark mode
* Responsive UI
* Enable or disable signups
* Restrict or unrestrict snippets visibility to anonymous users
* Admin panel : 
  * delete users/gists; 
  * clean database/filesystem by syncing gists
  * run `git gc` for all repositories
* SQLite/PostgreSQL/MySQL database
* Logging
* Docker support


## System requirements

[Git](https://git-scm.com/download) is obviously required to run Opengist, as it's the main feature of the app.
Version **2.28** or later is recommended as the app has not been tested with older Git versions and some features would not work.

[OpenSSH](https://www.openssh.com/) suite if you wish to use Git over SSH.


## Components

* Backend Web Framework: [Echo](https://echo.labstack.com/)
* ORM: [GORM](https://gorm.io/)
* Frontend libraries:
  * [TailwindCSS](https://tailwindcss.com/)
  * [CodeMirror](https://codemirror.net/)
  * [Day.js](https://day.js.org/)
  * and [others](/package.json)
