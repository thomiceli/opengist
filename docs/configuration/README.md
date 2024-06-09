# Configuration

Opengist provides flexible configuration options through either a YAML file and/or environment variables.
You would only need to specify the configuration options you want to change — for any config option left untouched, 
Opengist will simply apply the default values.

The [configuration cheat sheet](cheat-sheet.md) lists all available configuration options.

## Table of contents

- [Cheat sheet](cheat-sheet.md)
- [Admin panel](admin-panel.md)
- [OAuth providers](oauth-providers.md)
- [Password reset](reset-password.md)
- [Custom assets](custom-assets.md)
- [Custom links](custom-links.md)
- [Fail2ban](fail2ban.md)
- [Running using `systemd`](run-with-systemd.md)
- [Running behind reverse proxy](proxy.md)
- [Health check](healthcheck.md)


## Configuration via YAML file

The configuration file must be specified when launching the application, using the `--config` flag followed by the path 
to your YAML file.

Usage with Docker Compose :
```yml
services:
  opengist:
    # ...
    volumes:
    # ...
    - "/path/to/config.yml:/config.yml"
```

Usage via command line :
```shell
./opengist --config /path/to/config.yml
```

You can start by copying and/or modifying the provided [config.yml](/config.yml) file.


## Configuration via Environment Variables

Usage with Docker Compose :
```yml
services:
  opengist:
    # ...
    environment:
      OG_LOG_LEVEL: "info"
      # etc.
```

Usage via command line :
```shell
OG_LOG_LEVEL=info ./opengist
```
