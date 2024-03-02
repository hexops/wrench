#!/usr/bin/env bash
set -exuo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"/..

GOOS=linux GOARCH=amd64 go build
scp ./wrench root@pkg.machengine.org:/root/wrench-update
ssh root@pkg.machengine.org << EOF
  wrench svc stop
  mv /usr/local/bin/wrench /usr/local/bin/wrench-old
  mv /root/wrench-update /usr/local/bin/wrench
  chmod +x /usr/local/bin/wrench
  wrench svc start
  sleep 5
  wrench svc status
EOF

