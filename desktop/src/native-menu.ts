import type { MenuItemConstructorOptions } from "electron";

export interface NativeMenuActions {
  showInstances: () => void;
  saveDiagnosticReport: () => void;
}

export function buildNativeMenuTemplate(
  platform: NodeJS.Platform,
  appName: string,
  actions: NativeMenuActions,
): MenuItemConstructorOptions[] {
  const isMac = platform === "darwin";
  return [
    ...(isMac
      ? [
          {
            label: appName,
            submenu: [
              { role: "about" as const },
              { type: "separator" as const },
              { role: "services" as const },
              { type: "separator" as const },
              { role: "hide" as const },
              { role: "hideOthers" as const },
              { role: "unhide" as const },
              { type: "separator" as const },
              { role: "quit" as const },
            ],
          },
        ]
      : []),
    {
      label: "File",
      submenu: [
        {
          label: "Manage Instances…",
          accelerator: "CmdOrCtrl+Shift+L",
          click: actions.showInstances,
        },
        { type: "separator" },
        isMac ? { role: "close" } : { role: "quit" },
      ],
    },
    {
      label: "Edit",
      submenu: [
        { role: "undo" },
        { role: "redo" },
        { type: "separator" },
        { role: "cut" },
        { role: "copy" },
        { role: "paste" },
        { role: "pasteAndMatchStyle" },
        { role: "delete" },
        { type: "separator" },
        { role: "selectAll" },
      ],
    },
    {
      label: "View",
      submenu: [
        { role: "reload" },
        { type: "separator" },
        { role: "resetZoom" },
        { role: "zoomIn" },
        { role: "zoomOut" },
        { type: "separator" },
        { role: "togglefullscreen" },
      ],
    },
    {
      label: "Window",
      submenu: [
        { role: "minimize" },
        ...(isMac
          ? [
              { role: "zoom" as const },
              { type: "separator" as const },
              { role: "front" as const },
            ]
          : []),
      ],
    },
    {
      label: "Help",
      submenu: [
        {
          label: "Save Diagnostic Report…",
          click: actions.saveDiagnosticReport,
        },
        ...(!isMac
          ? [
              { type: "separator" as const },
              { role: "about" as const },
            ]
          : []),
      ],
    },
  ];
}
