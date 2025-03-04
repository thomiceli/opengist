# Metrics

Opengist offers built-in support for Prometheus metrics to help you monitor the performance and usage of your instance. These metrics provide insights into application health, user activity, and database statistics.

## Enabling metrics

By default, the metrics endpoint is disabled for security and performance reasons. To enable it, update your configuration as stated in the [configuration cheat sheet](cheat-sheet.md):

```yaml
metrics.enabled = true
```

Alternatively, you can use the environment variable:

```bash
OG_METRICS_ENABLED=true
```

Once enabled, metrics are available at the /metrics endpoint.

## Available metrics

### Opengist-specific metrics

| Metric Name | Type | Description |
|-------------|------|-------------|
| `opengist_users_total` | Gauge | Total number of registered users |
| `opengist_gists_total` | Gauge | Total number of gists in the system |
| `opengist_ssh_keys_total` | Gauge | Total number of SSH keys added by users |

### Standard HTTP metrics

In addition to the Opengist-specific metrics, standard Prometheus HTTP metrics are also available through the Echo Prometheus middleware. These include request durations, request counts, and request/response sizes.

These standard metrics follow the Prometheus naming convention and include labels for HTTP method, status code, and handler path.

## Security Considerations

The metrics endpoint exposes information about your Opengist instance that might be sensitive in some environments. Consider using a reverse proxy with authentication for the `/metrics` endpoint if your Opengist instance is publicly accessible.

Example with Nginx:

```shell
location /metrics {
    auth_basic "Metrics";
    auth_basic_user_file /etc/nginx/.htpasswd;
    proxy_pass http://localhost:6157/metrics;
}
```
