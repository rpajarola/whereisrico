# Debian/Ubuntu packaging

Builds `.deb` packages for `whereisricod` (the web server) and
`whereisricoctl` (the ingest/import CLI), with systemd units and a
dedicated `whereisrico` system user. Application data
(`whereisrico.db`, `gpx/`, `trips/`) lives under `/var/lib/whereisrico`,
entirely separate from the package's own files under `/usr` — it survives
package upgrades and removal (even `purge`; see `scripts/postremove.sh`).

## Build

```
go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest  # once
packaging/build.sh                                        # amd64 + arm64
```

Output: `dist/whereisrico_<version>_<arch>.deb`. Version defaults to the
current git tag, or `0.0.0~git<shortsha>` if untagged; override with
`VERSION=1.2.3 packaging/build.sh`. Build a single arch with
`ARCHES=amd64 packaging/build.sh`.

The Go binaries are built with `CGO_ENABLED=0` (the sqlite driver,
`modernc.org/sqlite`, is pure Go), so cross-compiling from any host,
including this one, needs no C toolchain.

## Install

```
sudo apt install ./dist/whereisrico_<version>_<arch>.deb
```

This creates the `whereisrico` system user, `/var/lib/whereisrico` (mode
0750, owned by that user), enables and starts `whereisricod.service`
(listening on `:8080` by default — see `/etc/default/whereisricod` to
change the port) and `whereisricoctl-ingest.timer` (runs `whereisricoctl
ingest` once daily to pick up any new `.gpx`/`.textproto` files dropped
into `/var/lib/whereisrico`).

Put a reverse proxy (nginx, Caddy, etc.) in front for TLS/a real domain —
`whereisricod` itself only speaks plain HTTP.

## Layout installed

```
/usr/bin/whereisricod
/usr/bin/whereisricoctl
/usr/share/whereisrico/static/            # web frontend
/lib/systemd/system/whereisricod.service
/lib/systemd/system/whereisricoctl-ingest.{service,timer}
/etc/default/whereisricod                 # conffile: WHEREISRICO_ADDR
/var/lib/whereisrico/                     # data (not owned by the package's lifecycle)
/var/lib/whereisrico/whereisrico.db
/var/lib/whereisrico/trips/*.textproto
/var/lib/whereisrico/gpx/*.gpx
```

## Operating

```
sudo systemctl status whereisricod
sudo journalctl -u whereisricod -f
sudo systemctl start whereisricoctl-ingest.service   # ingest on demand
```

To seed a fresh install with existing data, clone the private
[rpajarola/whereisrico-data](https://github.com/rpajarola/whereisrico-data)
repo and copy it in (ownership will be fixed by the next `whereisricoctl
ingest` run, or fix it directly):

```
git clone git@github.com:rpajarola/whereisrico-data.git /tmp/whereisrico-data
sudo cp -r /tmp/whereisrico-data/data/trips/*.textproto /var/lib/whereisrico/trips/
sudo cp -r /tmp/whereisrico-data/data/gpx/*.gpx /var/lib/whereisrico/gpx/
sudo cp /tmp/whereisrico-data/data/whereisrico.db /var/lib/whereisrico/   # or use whereisricoctl import-legacy
sudo chown -R whereisrico:whereisrico /var/lib/whereisrico
sudo systemctl restart whereisricod
```
