#!/bin/sh
# Runs before dpkg unpacks package files. The whereisrico system user/group
# must exist before unpacking so that the package's /var/lib/whereisrico
# directory entries (owner: whereisrico) can be chowned correctly as they
# are laid down.
set -e

if ! getent group whereisrico >/dev/null 2>&1; then
  addgroup --system whereisrico
fi

if ! getent passwd whereisrico >/dev/null 2>&1; then
  adduser --system --ingroup whereisrico --home /var/lib/whereisrico \
    --no-create-home --shell /usr/sbin/nologin whereisrico
fi

exit 0
