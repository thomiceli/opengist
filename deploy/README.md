# kustomize

## Simple

`kustomization.yaml`:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
metadata:
  name: opengist

resources:
  - https://github.com/thomiceli/opengist/deploy/
```

## Full example

`kustomization.yaml`:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
metadata:
  name: opengist

namespace: opengist

resources:
  - namespace.yaml
  - https://github.com/thomiceli/opengist/deploy/?ref:v1.7.3

images:
  - name: ghcr.io/thomiceli/opengist
    newTag: 1.7.3

patches:
  # Add your ingress
  - path: ingress.yaml
  - patch: |-
      - op: add
        path: /spec/rules/0/host
        value: opengist.mydomain.com
    target:
      group: networking.k8s.io
      version: v1
      kind: Ingress
      name: opengist
```

`namespace.yaml`:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: opengist
```

`ingress.yaml`:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: opengist
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-production
spec:
  ingressClassName: nginx
  tls:
    - hosts:
        - opengist.mydomain.com
      secretName: opengist-tls
```
