#!/bin/bash
set -e
# set -x

echo "Starting Mysqldump..."
schelly-mysql \
    --log-level=$LOG_LEVEL
