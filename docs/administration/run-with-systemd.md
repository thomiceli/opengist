# Run with Systemd

For non-Docker users, you could run Opengist as a systemd service.

On Unix distributions with systemd, place the Opengist binary like:

```shell
sudo cp opengist /usr/local/bin
sudo mkdir -p /var/lib/opengist
sudo cp config.yml /etc/opengist
```

Edit the Opengist home directory configuration in `/etc/opengist/config.yml` like:
```shell
opengist-home: /var/lib/opengist
```

Create a new user to run Opengist:
```shell
sudo useradd --system opengist
sudo mkdir -p /var/lib/opengist
sudo chown -R opengist:opengist /var/lib/opengist
```

Then create a service file at `/etc/systemd/system/opengist.service`:
```ini
[Unit]
Description=opengist Server
After=network.target

[Service]
Type=simple
User=opengist
Group=opengist
ExecStart=opengist --config /etc/opengist/config.yml
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

Finally, start the service:
```shell
systemctl daemon-reload
systemctl enable --now opengist
systemctl status opengist
```
