#!/bin/sh
# Rebuilds the embedded interface. Run after changing anything under web/.
set -e
cd "$(dirname "$0")/web"
npm ci --silent
npm run build
rm -rf ../internal/api/panel
cp -r dist ../internal/api/panel
echo "panel rebuilt into internal/api/panel"
