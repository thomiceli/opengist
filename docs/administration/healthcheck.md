# Healthcheck

A healthcheck is a simple HTTP GET request to the `/healthcheck` endpoint. It returns a `200 OK` response if the server is healthy.

## Example

```shell
curl http://localhost:6157/healthcheck
```

```json
{"database":"ok","opengist":"ok","time":"2024-01-04T05:18:33+01:00"}
```

## `HEAD /healthcheck`

A `HEAD` request to `/healthcheck` returns the same status code as the `GET`
request above, but without a body. This is handy for uptime monitors and load
balancers that probe the endpoint with `curl -I`:

```shell
curl -I http://localhost:6157/healthcheck
```
