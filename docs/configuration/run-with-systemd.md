# Run with Systemd

For non-Docker users, you could run Opengist as a systemd service.

## As root
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

----

## As a normal user
**NOTE: This was tested on Ubuntu 20.04 and newer. For other distros, please check the respective documentation**  

#### For the purpose of this documentation, we will assume that:
- You've followed the instructions on how to run opengist [from source](https://github.com/thomiceli/opengist?tab=readme-ov-file#from-source)
- Your shell user is named `pastebin`
- All commands are being executed as the `pastebin` user

_If none of the above is true, then adapt the commands and paths to fit your needs._

Enable lingering for the user:
```shell
loginctl enable-linger
```

Create the user systemd folder:
```
mkdir -p /home/pastebin/.config/systemd/user
```

Then create a service file at `/home/pastebin/.config/systemd/user/opengist.service`:
```ini
[Unit]
Description=opengist Server
After=network.target

[Service]
Type=simple
ExecStart=/home/pastebin/opengist/opengist --config /home/pastebin/opengist/config.yml
Restart=on-failure

[Install]
WantedBy=default.target
```

Finally, start the service:
```shell
systemctl --user daemon-reload
systemctl --user enable --now opengist
systemctl --user status opengist
```
