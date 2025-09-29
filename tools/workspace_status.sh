#!/bin/bash
echo "STABLE_WORKSPACE_HASH $(dirname $(dirname $(realpath "$0" | sed 's:^/home/builder::')) | md5sum | cut -c -8)"
