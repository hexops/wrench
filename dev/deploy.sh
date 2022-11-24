#!/usr/bin/env bash
set -exuo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"/..

source ./dev/remote.sh

DATE=$(date)
GOVERSION=$(go version)
VERSION=$(git describe --tags --abbrev=8 --dirty --always --long)
COMMIT_TITLE=$(git log --pretty=format:%s HEAD^1..HEAD)
PREFIX="github.com/hexops/wrench/internal/wrench"
LDFLAGS="-X '$PREFIX.Version=$VERSION'"
LDFLAGS="$LDFLAGS -X '$PREFIX.CommitTitle=$COMMIT_TITLE'"
LDFLAGS="$LDFLAGS -X '$PREFIX.Date=$DATE'"
LDFLAGS="$LDFLAGS -X '$PREFIX.GoVersion=$GOVERSION'"
GOARCH="amd64" GOOS="linux" go build -ldflags "$LDFLAGS" -o bin/wrench .

ssh $remote -f 'sudo systemctl stop wrench'

while ! scp ./bin/wrench $remote:/usr/local/bin/wrench
do
    echo 'waiting to scp'
    sleep 1
done

scp ./dev/systemd/wrench.service $remote:/etc/systemd/system/wrench.service
scp ./dev/systemd/wrench-start.sh $remote:/usr/local/bin/wrench-start.sh
scp -r ../wrench-private/ssh $remote:/root/.ssh
scp -r ../wrench-private/config.toml $remote:/root/config.toml

ssh $remote << EOF
  set -exuo pipefail
  mkdir -p $HOME/wrench
  sudo chmod 744 /usr/local/bin/wrench
  sudo chmod 744 /usr/local/bin/wrench-start.sh
  sudo chmod 664 /etc/systemd/system/wrench.service
  sudo systemctl daemon-reload
  sudo systemctl enable wrench.service
  sudo systemctl start wrench
EOF

echo "Wrench has been deployed!"