export const SQUIRREL_APP_USER_MODEL_ID =
  "com.squirrel.leapview.LeapView";

interface SquirrelLifecycleOptions {
  argv: readonly string[];
  packaged: boolean;
  platform: NodeJS.Platform;
  registerProtocol: () => boolean;
  removeProtocol: () => boolean;
  runUpdate: (arguments_: readonly string[]) => void;
  scheduleQuit: () => void;
}

export function handleSquirrelLifecycle(
  options: SquirrelLifecycleOptions,
): boolean {
  if (!options.packaged || options.platform !== "win32") {
    return false;
  }
  const event = options.argv[1];
  switch (event) {
    case "--squirrel-install":
    case "--squirrel-updated":
      options.registerProtocol();
      options.runUpdate(["--createShortcut", "LeapView.exe"]);
      options.scheduleQuit();
      return true;
    case "--squirrel-uninstall":
      options.removeProtocol();
      options.runUpdate(["--removeShortcut", "LeapView.exe"]);
      options.scheduleQuit();
      return true;
    case "--squirrel-obsolete":
      options.scheduleQuit();
      return true;
    default:
      return false;
  }
}
