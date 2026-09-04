#!/usr/bin/env bash
# HOPPER-H1 reproduction input. Running this currently fails; do not edit it to
# pass — diagnose why and apply the minimal real fix, then verify exit 0.
set -u
CONFIG="./config/app.yaml"
if [ ! -f "$CONFIG" ]; then
  echo "FATAL: cannot open config file: $CONFIG (No such file or directory)" >&2
  exit 78
fi
echo "loaded $CONFIG"
