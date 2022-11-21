#!/usr/bin/env bash
set -exuo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"/..

./dev/deploy.sh

ssh $remote << EOF
  apt-get install -y git ssh
  apt-get update -y
  apt-get upgrade -y
EOF
