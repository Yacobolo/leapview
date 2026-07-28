#!/bin/sh
set -eu

display_number=99
Xvfb ":${display_number}" -screen 0 1280x720x24 -nolisten tcp &
export DISPLAY=":${display_number}"

attempt=0
while [ ! -S "/tmp/.X11-unix/X${display_number}" ]; do
	attempt=$((attempt + 1))
	if [ "$attempt" -ge 50 ]; then
		echo "Xvfb did not become ready" >&2
		exit 1
	fi
	sleep 0.1
done

exec go test -count=1 \
	-run TestElectronCandidatePreservesBrowserEquivalentAuthority \
	-v ./internal/app/testing/maliciousinstance
