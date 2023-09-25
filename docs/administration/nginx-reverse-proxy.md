# Use Nginx as a reverse proxy

Configure Nginx to proxy requests to Opengist. Here is an example configuration file :
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

Then run :
```shell
service nginx restart
```
