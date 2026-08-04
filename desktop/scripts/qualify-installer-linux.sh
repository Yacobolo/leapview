#!/bin/sh
set -eu

if [ "${GITHUB_ACTIONS:-}" != "true" ] || [ "$#" -ne 1 ]; then
  echo "usage: run only in GitHub Actions with one Debian package" >&2
  exit 1
fi

artifact_directory="$(cd "$(dirname "$1")" && pwd)"
artifact="${artifact_directory}/$(basename "$1")"
desktop_entry="/usr/share/applications/leapview-desktop.desktop"

sudo apt-get install --yes "${artifact}"
test -f "${desktop_entry}"
grep -Fx "Exec=leapview-desktop %U" "${desktop_entry}"
grep -Fx "MimeType=x-scheme-handler/leapview-desktop;" "${desktop_entry}"

sudo apt-get install --yes --reinstall "${artifact}"
xdg-mime default leapview-desktop.desktop x-scheme-handler/leapview-desktop
test "$(xdg-mime query default x-scheme-handler/leapview-desktop)" = \
  "leapview-desktop.desktop"

sudo dpkg --remove leapview-desktop
test ! -e "${desktop_entry}"
