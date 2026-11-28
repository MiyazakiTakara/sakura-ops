# SakuraOps development PostgreSQL image: the official image minus gosu, non-root.
# Build context is the repository root. The base is pinned by digest; the tag is a comment.
#
# Why derived: the official entrypoint starts as root only to chown PGDATA and then
# drops privileges with gosu (a Go binary whose bundled standard library carries known
# vulnerabilities). SakuraOps never starts PostgreSQL as root, so the entrypoint never
# takes that branch and gosu is dead weight. It is removed from the image instead of
# being ignored in the scan.
#
# Why POSTGRES_UID/POSTGRES_GID: Compose (outside Swarm) bind-mounts secret files with
# the host owner and mode (0600, developer uid). The container must therefore run as
# that uid to read POSTGRES_PASSWORD_FILE, and a fresh named volume takes ownership from
# the image directory. The build re-numbers the image's postgres user to the uid:gid
# passed by Compose (SAKURAOPS_DEV_UID/GID), so initdb finds a passwd entry for the
# running uid and /var/lib/postgresql is owned by it, all without root at runtime.
# Only the ownership of two directories depends on the build args; package content
# does not, so the default (1000:1000) is what the image scan builds.

# postgres:18-trixie (PostgreSQL 18, current supported major)
FROM postgres@sha256:64c8b0d31e4ab6ea665e8426ea406a94bf52de3c05b3a4a8bf713704eefa9328

ARG POSTGRES_UID=1000
ARG POSTGRES_GID=1000

RUN set -eux; \
    case "${POSTGRES_UID}:${POSTGRES_GID}" in \
      *[!0-9:]*|0:*|*:0|:*|*:) echo "POSTGRES_UID/POSTGRES_GID must be positive integers, not 0" >&2; exit 1 ;; \
    esac; \
    rm -f /usr/local/bin/gosu; \
    if [ "$(id -g postgres)" != "${POSTGRES_GID}" ]; then groupmod --non-unique --gid "${POSTGRES_GID}" postgres; fi; \
    if [ "$(id -u postgres)" != "${POSTGRES_UID}" ]; then usermod --non-unique --uid "${POSTGRES_UID}" --gid "${POSTGRES_GID}" postgres; fi; \
    mkdir -p /var/lib/postgresql /var/run/postgresql; \
    chown -R postgres:postgres /var/lib/postgresql /var/run/postgresql; \
    test ! -e /usr/local/bin/gosu; \
    test "$(id -u postgres)" = "${POSTGRES_UID}"

USER postgres
