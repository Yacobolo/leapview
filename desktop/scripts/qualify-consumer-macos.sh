#!/bin/sh
set -eu

if [ "${GITHUB_ACTIONS:-}" != "true" ] || [ "$#" -ne 1 ]; then
  echo "usage: run only in GitHub Actions with one macOS DMG" >&2
  exit 1
fi

artifact="$1"
temporary="$(mktemp -d)"
mount="${temporary}/mount"
application_root="${temporary}/Applications"
application="${application_root}/LeapView.app"
bundle_id="dev.leapview.desktop"
launch_services="/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister"

cleanup() {
  "${launch_services}" -u "${application}" >/dev/null 2>&1 || true
  hdiutil detach "${mount}" >/dev/null 2>&1 || true
  rm -rf "${temporary}"
}
trap cleanup EXIT

mkdir "${mount}" "${application_root}"
hdiutil attach -readonly -nobrowse -mountpoint "${mount}" "${artifact}"
test -x "${mount}/LeapView.app/Contents/MacOS/LeapView"
cp -R "${mount}/LeapView.app" "${application}"
test -x "${application}/Contents/MacOS/LeapView"

installed_bundle_id="$(
  plutil -extract CFBundleIdentifier raw \
    "${application}/Contents/Info.plist"
)"
test "${installed_bundle_id}" = "${bundle_id}"
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

rm -rf "${application}"
test ! -e "${application}"
