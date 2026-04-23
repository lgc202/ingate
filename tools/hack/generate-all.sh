#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/../.."

./tools/hack/generate-apis.sh
./tools/hack/generate-clients.sh
./tools/hack/generate-proto.sh
