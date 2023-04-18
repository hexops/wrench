#!/usr/bin/env bash
set -exuo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"/..

./dev/deploy.sh

ssh $remote << EOF
  apt-get install -y git ssh
  apt-get update -y
  apt-get upgrade -y

  curl -OL https://go.dev/dl/go1.20.3.linux-amd64.tar.gz
  sudo rm -rf /usr/local/go
  sudo tar -C /usr/local -xzf go1.20.3.linux-amd64.tar.gz
  sudo ln -s /usr/local/go/bin/go /usr/local/bin/go

  sudo swapoff /swapfile || true
  sudo rm /swapfile || true
  sudo dd if=/dev/zero of=/swapfile bs=1M count=1024
  sudo chmod 600 /swapfile
  sudo mkswap /swapfile
  sudo swapon /swapfile
EOF
