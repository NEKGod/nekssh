#!/bin/sh
set -e

if ! getent group nekssh >/dev/null 2>&1; then
    groupadd --system nekssh
fi
if ! getent passwd nekssh >/dev/null 2>&1; then
    useradd --system --gid nekssh --home-dir /var/lib/nekssh --shell /sbin/nologin nekssh
fi
install -d -m 0750 -o nekssh -g nekssh /var/lib/nekssh
touch /var/lib/nekssh/known_hosts
chown nekssh:nekssh /var/lib/nekssh/known_hosts
chmod 0600 /var/lib/nekssh/known_hosts

systemctl daemon-reload >/dev/null 2>&1 || true
systemctl enable --now nekssh.service >/dev/null 2>&1 || true

if command -v firewall-cmd >/dev/null 2>&1 && firewall-cmd --state >/dev/null 2>&1; then
    firewall-cmd --zone=public --add-port=8022/tcp --permanent >/dev/null 2>&1 || true
    firewall-cmd --reload >/dev/null 2>&1 || true
fi
