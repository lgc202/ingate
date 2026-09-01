#!/bin/sh
set -eu

address=${TEMPORAL_ADDRESS:-temporal:7233}
namespace=${TEMPORAL_NAMESPACE:-ingate}
attempt=1

while [ "$attempt" -le 30 ]; do
  if temporal operator cluster health \
    --address "$address" \
    --client-connect-timeout 3s \
    --command-timeout 5s >/dev/null 2>&1; then
    break
  fi
  attempt=$((attempt + 1))
  sleep 2
done

if [ "$attempt" -gt 30 ]; then
  echo "Temporal service did not become ready" >&2
  exit 1
fi

if temporal operator namespace describe \
  --address "$address" \
  --namespace "$namespace" \
  --client-connect-timeout 3s \
  --command-timeout 5s >/dev/null 2>&1; then
  exit 0
fi

if temporal operator namespace create \
  --address "$address" \
  --namespace "$namespace" \
  --client-connect-timeout 3s \
  --command-timeout 5s >/dev/null 2>&1; then
  exit 0
fi

echo "failed to create Temporal namespace" >&2
exit 1
