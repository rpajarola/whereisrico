#!/bin/sh
# Runs before removing package files, on both removal and upgrade. Stopping
# unconditionally is simple and safe: on upgrade, the new version's
# postinstall re-enables and starts everything again.
set -e

if [ -d /run/systemd/system ]; then
  systemctl disable --now whereisricod.service || true
  systemctl disable --now whereisricoctl-ingest.timer || true
fi

exit 0
