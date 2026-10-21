#!/usr/bin/env bash
# Compiles an OCSF schema release into validate/schema/ocsf-schema-v<version>.json
# with the official OCSF schema compiler (pinned), for embedding and code
# generation. Needs git and python3. Usage: scripts/compile-schema.sh 1.5.0
set -euo pipefail

version=${1:?usage: $0 <ocsf version, e.g. 1.5.0>}
compiler=ocsf-schema-compiler==1.1.1
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

git clone -q --depth 1 --branch "v$version" https://github.com/ocsf/ocsf-schema "$work/schema"
python3 -m venv "$work/venv"
"$work/venv/bin/pip" install -q "$compiler"
out=validate/schema/ocsf-schema-v$version.json
"$work/venv/bin/ocsf-schema-compiler" "$work/schema" >"$out"
echo "wrote $out ($(wc -c <"$out" | tr -d ' ') bytes) with $compiler"
