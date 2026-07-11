#!/bin/sh
systemctl disable --now nekssh.service >/dev/null 2>&1 || true
