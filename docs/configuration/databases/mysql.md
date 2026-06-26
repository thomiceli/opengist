# Using MySQL/MariaDB

To use MySQL/MariaDB as the database backend, you need to set the database URI configuration to the connection string of your MySQL/MariaDB database with this format :

`mysql://<user>:<password>@<host>:<port>/<database>`

#### YAML
```yaml
# Example
db-uri: mysql://root:passwd@localhost:3306/opengist_db
```

#### Environment variable
```sh
# Example
OG_DB_URI=mysql://root:passwd@localhost:3306/opengist_db
```

### Unix socket

To connect through a Unix socket instead of TCP, omit the host/port and set the `socket` query parameter to the socket path:

`mysql://<user>:<password>@/<database>?socket=<socket-path>`

```yaml
# Example
db-uri: mysql://root:passwd@/opengist_db?socket=/var/run/mysqld/mysqld.sock
```

### Docker Compose
```yml
services:
  opengist:
    image: ghcr.io/thomiceli/opengist:1
    container_name: opengist
    restart: unless-stopped
    depends_on:
      - mysql
    ports:
      - "6157:6157"
      - "2222:2222"
    volumes:
      - "$HOME/.opengist:/opengist"
    environment:
      OG_DB_URI: mysql://opengist:secret@mysql:3306/opengist_db
      # other configuration options

  mysql:
    image: mysql:8.4
    restart: unless-stopped
    volumes:
      - "./opengist-database:/var/lib/mysql"
    environment:
      MYSQL_USER: opengist
      MYSQL_PASSWORD: secret
      MYSQL_DATABASE: opengist_db
      MYSQL_ROOT_PASSWORD: rootsecret
```