# Helm Chart Changelog

## 0.6.0 - 2026-02-03

- Bump Opengist image to 1.12.1

## 0.5.0 - 2026-01-27

- Bump Opengist image to 1.12.0
- Add StatefulSet support
- Add Prometheus ServiceMonitor support if Opengist metrics are enabled
- New service for metrics endpoint, dissociated from the main service
- Use existing pvc claim of provided

## 0.4.0 - 2025-09-30

- Bump Opengist image to 1.11.1

## 0.3.0 - 2025-09-21

- Bump Opengist image to 1.11.0

## 0.2.0 - 2025-05-10

- Add `deployment.env[]` in values

## 0.1.0 - 2025-04-06

- Initial release, with Opengist image 1.10.0
