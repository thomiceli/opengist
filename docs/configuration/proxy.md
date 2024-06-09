# Reverse proxy

Configure Traefik or Nginx to proxy requests to Opengist.

> [!Important]
> Make sure you set the base url for Opengist via the [configuration](/docs/configuration/cheat-sheet.md).

## Traefik

You can set up Traefik in two ways:

<details>
<summary>Using Docker labels</summary>

Add these labels to your `docker-compose.yml` file:

```yml
    labels:
      - traefik.http.routers.opengist.rule=Host(`opengist.example.com`) # Change to your subdomain
      # Uncomment the line below if you run Opengist in a subdirectory
      # - traefik.http.routers.app1.rule=PathPrefix(`/opengist{regex:$$|/.*}`) # Change opentist in the regex to yuor subdirectory name
      - traefik.http.routers.opengist.entrypoints=websecure # Change to the name of your 443 port entrypoint
      - traefik.http.routers.opengist.tls.certresolver=lets-encrypt # Change to certresolver's name
      - traefik.http.routers.opengist.service=opengist
      - traefik.http.services.opengist.loadBalancer.server.port=6157
```

</details>


<details>
<summary>Using a <code>yml</code> file</summary>

> ![Note]
> Don't forget to change the `<server-address>` to your server's IP

`traefik_dynamic.yml`
```yml
http:
  routers:
    opengist:
      entrypoints: websecure
      rule: Host(`opengist.example.com`) # Comment this line and uncomment the line below if using a subpath
      # rule: PathPrefix(`/opengist{regex:$$|/.*}`) # Change opentist in the regex to yuor subdirectory name
      # middlewares:
      #   - opengist-fail2ban
      service: opengist
      tls:
        certresolver: lets-encrypt
  services:
    opengist:
      loadbalancer:
        servers:
          - url: "http://<server-address>:6157"

```

</details>

## Nginx


### Subdomain
```
server {
    listen 80;
    server_name opengist.example.com;

    location / {
        proxy_pass http://127.0.0.1:6157;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

### Subpath
```
server {
    listen 80;
    server_name example.com;

    location /opengist/ {
        rewrite ^/opengist(/.*)$ $1 break;
        proxy_pass http://127.0.0.1:6157;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-Prefix /opengist;
    }
}
```

---

To apply changes:
```shell
sudo systemctl restart nginx
```
