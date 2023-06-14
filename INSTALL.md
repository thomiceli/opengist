# Install Opengist

Install Go and NodeJS:

```bash
wget https://go.dev/dl/go1.20.5.linux-amd64.tar.gz
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.20.5.linux-amd64.tar.gz
sudo apt update; sudo apt install -y curl; curl -fsSL https://deb.nodesource.com/setup_18.x | sudo -E bash - && sudo apt-get install -y nodejs
sudo apt install gcc
```



Clone the repository and build:
```bash
git clone https://github.com/thomiceli/opengist
cd opengist
make
```



Install Opengist:

```bash
sudo cp opengist /usr/local/bin
sudo mkdir -p /var/lib/opengist
sudo cp config.yml /etc/opengist
```



Edit `/etc/opengist/config.yml`:

```yaml
opengist-home: /var/lib/opengist
```



Create a new user to run Opengist:

```bash
sudo useradd --system opengist
sudo mkdir -p /var/lib/opengist
sudo chown -R opengist:opengist /var/lib/opengist
```



Create a systemd service:

```bash
cat >/etc/systemd/system/opengist.service <<EOF
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
EOF

systemctl daemon-reload
systemctl enable --now opengist
systemctl status opengist
```
