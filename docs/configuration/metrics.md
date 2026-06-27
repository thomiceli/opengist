# Metrics

Opengist offers built-in support for Prometheus metrics to help you monitor the performance and usage of your instance. These metrics provide insights into application health, user activity, and database statistics.

## Enabling metrics

By default, the metrics server is disabled for security and performance reasons. To enable it, update your configuration as stated in the [configuration cheat sheet](cheat-sheet.md):

```yaml
metrics.enabled: true
```

Alternatively, you can use the environment variable:

```bash
OG_METRICS_ENABLED=true
```

Once enabled, metrics are available on a separate server at `http://0.0.0.0:6158/metrics` by default.

## Configuration

The metrics server runs on a separate port from the main application. By default, it binds to `0.0.0.0` (all interfaces) on port `6158`.

| Config Key     | Environment Variable | Default     | Description                                    |
|----------------|---------------------|-------------|------------------------------------------------|
| metrics.enabled | OG_METRICS_ENABLED  | `false`     | Enable or disable the metrics server           |
| metrics.host    | OG_METRICS_HOST     | `0.0.0.0`   | The host on which the metrics server binds     |
| metrics.port    | OG_METRICS_PORT     | `6158`      | The port on which the metrics server listens   |

Example configuration:

```yaml
metrics.enabled: true
metrics.host: 0.0.0.0
metrics.port: 6158
```

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

The metrics server binds to `0.0.0.0` by default, making it accessible on all network interfaces. This default works well for containerized deployments (Docker, Kubernetes) where network isolation is handled at the infrastructure level.

For bare-metal or VM deployments where the metrics port may be exposed, consider restricting to localhost by setting `metrics.host: 127.0.0.1` to only allow local access.
