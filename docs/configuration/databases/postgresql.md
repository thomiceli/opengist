# Using PostgreSQL

To use PostgreSQL as the database backend, you need to set the database URI configuration to the connection string of your PostgreSQL database with this format :

`postgres://<user>:<password>@<host>:<port>/<database>`

#### YAML
```yaml
# Example
db-uri: postgres://postgres:passwd@localhost:5432/opengist_db
```

#### Environment variable
```sh
# Example
OG_DB_URI=postgres://postgres:passwd@localhost:5432/opengist_db
```

### Docker Compose
```yml
services:
  opengist:
    image: ghcr.io/thomiceli/opengist:1
    container_name: opengist
    restart: unless-stopped
    depends_on:
      - postgres
    ports:
      - "6157:6157"
      - "2222:2222"
    volumes:
      - "$HOME/.opengist:/opengist"
    environment:
      OG_DB_URI: postgres://opengist:secret@postgres:5432/opengist_db
      # other configuration options

  postgres:
    image: postgres:16.4
    restart: unless-stopped
    volumes:
      - "./opengist-database:/var/lib/postgresql/data"
    environment:
      POSTGRES_USER: opengist
      POSTGRES_PASSWORD: secret
      POSTGRES_DB: opengist_db
```