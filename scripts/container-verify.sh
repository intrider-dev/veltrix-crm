#!/usr/bin/env sh
set -eu

image="${1:-crm-app:local}"
user="$(docker image inspect --format '{{.Config.User}}' "$image")"
if [ "$user" != "65532:65532" ]; then
  echo "unexpected runtime user: $user" >&2
  exit 1
fi

if docker run --rm --entrypoint /usr/local/bin/node "$image" --version >/dev/null 2>&1; then
  echo "Node.js was found in the runtime image" >&2
  exit 1
fi

docker image inspect --format '{{json .Config.Healthcheck.Test}}' "$image"
echo "runtime user and Node.js absence checks passed"
