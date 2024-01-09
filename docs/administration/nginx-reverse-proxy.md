# Use Nginx as a reverse proxy

Configure Nginx to proxy requests to Opengist. Here are example configuration file to use Opengist on a subdomain or on a subpath.

Make sure you set the base url for Opengist via the [configuration](/docs/configuration/cheat-sheet.md).

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
