# Healthcheck

A healthcheck is a simple HTTP GET request to the `/healthcheck` endpoint. It returns a `200 OK` response if the server is healthy.

> [!Note]
> Don't forget to change the port from `6157` in the below examples
if you changed it in the configuration.

## `docker-compose`

```yml
    healthcheck:
      test:  curl -sS http://localhost:6157/healthcheck | grep '"database":"ok"' | grep '"opengist":"ok"' || exit 1
      interval: 10s
      timeout: 5s
      retries: 10
```

## Example

```shell
curl http://localhost:6157/healthcheck
```

```json
{"database":"ok","opengist":"ok","time":"2024-01-04T05:18:33+01:00"}
```
