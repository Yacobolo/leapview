#!/bin/sh
set -eu

if [ "${GITHUB_ACTIONS:-}" != "true" ] || [ "$#" -ne 1 ]; then
  echo "usage: run only in GitHub Actions with one macOS DMG" >&2
  exit 1
fi

artifact="$1"
temporary="$(cd "$(mktemp -d)" && pwd -P)"
mount="${temporary}/mount"
application_root="${HOME}/Applications"
application="${application_root}/LeapView.app"
bundle_id="dev.leapview.desktop"
launch_services="/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister"

if [ -e "${application}" ]; then
  echo "qualification refuses to replace an existing LeapView app" >&2
  exit 1
fi
cleanup() {
  "${launch_services}" -u "${application}" >/dev/null 2>&1 || true
  rm -rf "${application}"
  hdiutil detach "${mount}" >/dev/null 2>&1 || true
  rm -rf "${temporary}"
}
trap cleanup EXIT

mkdir "${mount}"
mkdir -p "${application_root}"
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
protocol_registered="$(
  osascript -l JavaScript -e '
    function run(argv) {
      ObjC.import("AppKit");
      const url = $.NSURL.URLWithString("leapview-desktop://connect");
      const applications = $.NSWorkspace.sharedWorkspace
        .URLsForApplicationsToOpenURL(url);
      const paths = ObjC.deepUnwrap(applications.valueForKey("path"));
      return paths.includes(argv[0]) ? "registered" : "missing";
    }
  ' \
    -- \
    "${application}"
)"
test "${protocol_registered}" = "registered"

rm -rf "${application}"
test ! -e "${application}"
