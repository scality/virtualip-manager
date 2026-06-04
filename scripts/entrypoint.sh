#!/bin/bash

set -xue -o pipefail

/generate-config -input "$1" -output "/etc/keepalived/keepalived.conf"

exec keepalived \
    --log-console --log-detail \
    --dont-fork --dont-respawn \
    --address-monitoring \
    --dump-conf --use-file "/etc/keepalived/keepalived.conf"
