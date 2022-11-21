#!/usr/bin/env bash
set -exuo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"/..

source ./dev/remote.sh

ssh $remote -f 'journalctl -u wrench.service'
