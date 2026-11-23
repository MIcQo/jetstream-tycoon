#!/bin/sh
# Creates the saves bucket and expires abandoned uploads under staging/ after 1 day (ADR-0023).
set -eu
mc alias set local http://minio:9000 jetstream jetstream-dev-secret
mc mb --ignore-existing local/jetstream-saves
mc ilm rule add --prefix "staging/" --expire-days 1 local/jetstream-saves || true
mc ilm rule ls local/jetstream-saves
