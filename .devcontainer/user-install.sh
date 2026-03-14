#!/bin/bash
set -x

curl -fsSL https://opencode.ai/install | bash 
curl -fsSL https://claude.ai/install.sh | bash
curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.40.4/install.sh | bash

export NVM_DIR="$HOME/.nvm"
[ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"  # This loads nvm
[ -s "$NVM_DIR/bash_completion" ] && \. "$NVM_DIR/bash_completion"  # This loads nvm bash_completion

nvm install node
npm install -g @openai/codex

mkdir /home/vscode/.codex
ln -s /home/vscode/.claude.settings.json /home/vscode/.claude/settings.json
ln -s /home/vscode/.codex.config.toml /home/vscode/.codex/config.toml
