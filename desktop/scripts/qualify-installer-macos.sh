#!/bin/sh
set -eu

if [ "${GITHUB_ACTIONS:-}" != "true" ] || [ "$#" -ne 1 ]; then
  echo "usage: run only in GitHub Actions with one macOS package" >&2
  exit 1
fi

artifact="$1"
application="/Applications/LeapView.app"
policy_directory="/Library/Application Support/LeapView"
policy_path="${policy_directory}/desktop-policy.json"
bundle_id="dev.leapview.desktop"
launch_services="/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister"

sudo installer -pkg "${artifact}" -target /
if [ ! -x "${application}/Contents/MacOS/LeapView" ]; then
  echo "installer did not place an executable LeapView app in /Applications" >&2
  pkgutil --files "${bundle_id}" >&2 || true
  find /Applications -maxdepth 3 -name 'LeapView.app' -print >&2
  exit 1
fi
policy_mode="$(stat -f '%Su:%Sg:%Lp' "${policy_directory}")"
if [ "${policy_mode}" != "root:wheel:755" ]; then
  echo "managed policy directory has unexpected ${policy_mode}" >&2
  exit 1
fi
installed_bundle_id="$(
  plutil -extract CFBundleIdentifier raw \
    "${application}/Contents/Info.plist"
)"
if [ "${installed_bundle_id}" != "${bundle_id}" ]; then
  echo "installed application has unexpected bundle ID" >&2
  exit 1
fi
plutil -extract CFBundleURLTypes xml1 -o - \
  "${application}/Contents/Info.plist" | grep -F "leapview-desktop"
"${launch_services}" -f "${application}"
registered_application="$(
  osascript -l JavaScript -e '
    ObjC.import("AppKit");
    const url = $.NSURL.URLWithString("leapview-desktop://connect");
    const application = $.NSWorkspace.sharedWorkspace
      .URLForApplicationToOpenURL(url);
    application ? ObjC.unwrap(application.path) : "";
  '
)"
test "${registered_application}" = "${application}"

sudo sh -c "printf '%s\n' '{\"qualification\":\"preserve-on-upgrade\"}' > '${policy_path}'"
sudo chown root:wheel "${policy_path}"
sudo chmod 0644 "${policy_path}"
if [ -w "${policy_path}" ]; then
  echo "standard user can modify managed policy" >&2
  exit 1
fi

sudo installer -pkg "${artifact}" -target /
grep -F '"qualification":"preserve-on-upgrade"' "${policy_path}"
test "$(stat -f '%Su:%Sg:%Lp' "${policy_path}")" = "root:wheel:644"

sudo rm -rf "/Applications/LeapView.app"
sudo pkgutil --forget "${bundle_id}"
test ! -e "${application}"
test -f "${policy_path}"
