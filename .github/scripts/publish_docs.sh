#!/usr/bin/env bash

set -e

git config --global user.email "bot@stealthrocket.tech"
git config --global user.name "Stealth Rocket Bot"

mkdir -p ~/.ssh
echo -e "${TIMECRAFT_DOCS_SSH_KEY}" > ~/.ssh/id_ed25519
chmod 600 ~/.ssh/id_ed25519

git clone git@github.com:stealthrocket/docs.timecraft.dev.git ~/docs.timecraft.dev

rsync -av docs/ ~/docs.timecraft.dev/docs

cd ~/docs.timecraft.dev

git add -A
git commit -am "Update docs from github.com/stealthrocket/timecraft@${GITHUB_SHA}"
git push origin main
