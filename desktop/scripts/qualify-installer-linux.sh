#!/bin/sh
set -eu

if [ "${GITHUB_ACTIONS:-}" != "true" ] || [ "$#" -ne 1 ]; then
  echo "usage: run only in GitHub Actions with one Debian package" >&2
  exit 1
fi

artifact="$1"
policy_directory="/etc/leapview"
policy_path="${policy_directory}/desktop-policy.json"
desktop_entry="/usr/share/applications/leapview-desktop.desktop"

sudo dpkg --install "${artifact}"
test "$(stat -c '%U:%G:%a' "${policy_directory}")" = "root:root:755"
test -f "${desktop_entry}"
grep -Fx "Exec=leapview-desktop %U" "${desktop_entry}"
grep -Fx "MimeType=x-scheme-handler/leapview-desktop;" "${desktop_entry}"

sudo sh -c "printf '%s\n' '{\"qualification\":\"preserve-on-upgrade\"}' > '${policy_path}'"
sudo chown root:root "${policy_path}"
sudo chmod 0644 "${policy_path}"
if [ -w "${policy_path}" ]; then
  echo "standard user can modify managed policy" >&2
  exit 1
fi

sudo dpkg --install "${artifact}"
grep -F '"qualification":"preserve-on-upgrade"' "${policy_path}"
test "$(stat -c '%U:%G:%a' "${policy_path}")" = "root:root:644"

xdg-mime default leapview-desktop.desktop x-scheme-handler/leapview-desktop
test "$(xdg-mime query default x-scheme-handler/leapview-desktop)" = \
  "leapview-desktop.desktop"

sudo dpkg --remove leapview-desktop
test ! -e "${desktop_entry}"
test -f "${policy_path}"
