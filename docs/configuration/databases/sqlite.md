# Using SQLite

By default, Opengist uses SQLite as the database backend.

Because SQLite is a file-based database, there is not much configuration to tweak.

The configuration `db-uri`/`OG_DB_URI` refers to the path of the SQLite database file relative in the `$opengist-home/` directory (default `opengist.db`),
although it can be left untouched.

The SQLite journal mode is set to [`WAL` (Write-Ahead Logging)](https://www.sqlite.org/pragma.html#pragma_journal_mode) by default and can be changed.

#### YAML
```yaml
sqlite.journal-mode: WAL
```

#### Environment variable
```sh
OG_SQLITE_JOURNAL_MODE=WAL
```

### Docker Compose
```yml
services:
  opengist:
    image: ghcr.io/thomiceli/opengist:1
    container_name: opengist
    restart: unless-stopped
    ports:
      - "6157:6157" # HTTP port
      - "2222:2222" # SSH port, can be removed if you don't use SSH
    volumes:
      - "$HOME/.opengist:/opengist"
    environment:
      OG_SQLITE_JOURNAL_MODE: WAL
      # other configuration options
```