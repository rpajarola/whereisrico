#!/bin/sh
# Runs after dpkg unpacks package files.
set -e

# Defensive: nfpm's dir-type contents entries already create these with the
# right ownership, but make sure regardless (e.g. if someone pre-created
# /var/lib/whereisrico by hand before installing).
mkdir -p /var/lib/whereisrico/trips /var/lib/whereisrico/gpx
chown -R whereisrico:whereisrico /var/lib/whereisrico

if [ -d /run/systemd/system ]; then
  systemctl daemon-reload
  systemctl enable --now whereisricod.service
  systemctl enable --now whereisricoctl-ingest.timer
fi

exit 0
