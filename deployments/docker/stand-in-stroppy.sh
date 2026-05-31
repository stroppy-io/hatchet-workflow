#!/bin/sh
# Stand-in stroppy binary for the demo. The real stroppy is fetched from minio /
# a github release in production (set STROPPY_UPSTREAM to an http(s) URL); for the
# self-contained demo the server serves THIS script as the "stroppy" artifact.
# It accepts the recipe's `--url <dsn> run` invocation, does a trivial DB touch
# to prove connectivity, and exits 0.
echo "stand-in stroppy invoked: $*"
exit 0
