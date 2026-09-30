#!/bin/sh
# Runs after removing package files. Intentionally does NOT remove
# /var/lib/whereisrico (the database, gpx/, trips/) or the whereisrico
# system user/group, even on purge: that data is meant to be kept separate
# from the package and survive reinstalls. Remove it yourself if you really
# want to:
#   sudo rm -rf /var/lib/whereisrico
#   sudo deluser whereisrico && sudo delgroup whereisrico
set -e

if [ "$1" = "purge" ] && [ -d /run/systemd/system ]; then
  systemctl daemon-reload || true
fi

exit 0
