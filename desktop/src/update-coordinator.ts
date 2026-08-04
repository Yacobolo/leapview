import type {
  MessageBoxOptions,
  MessageBoxReturnValue,
} from "electron";

import {
  DesktopUpdater,
  type DesktopAutoUpdater,
  type DesktopUpdateEvent,
  type DesktopUpdateRuntime,
} from "./updater.js";

const INITIAL_UPDATE_CHECK_DELAY_MS = 10_000;
const UPDATE_CHECK_INTERVAL_MS = 6 * 60 * 60 * 1_000;

export interface DesktopUpdateCoordinatorOptions {
  native: DesktopAutoUpdater;
  runtime: DesktopUpdateRuntime;
  showMessageBox(
    options: MessageBoxOptions,
  ): Promise<MessageBoxReturnValue>;
  recordEvent(event: DesktopUpdateEvent): void;
  beforeRestart(): Promise<void>;
}

export class DesktopUpdateCoordinator {
  readonly #platform: NodeJS.Platform;
  readonly #showMessageBox: DesktopUpdateCoordinatorOptions["showMessageBox"];
  readonly #recordEvent: DesktopUpdateCoordinatorOptions["recordEvent"];
  readonly #updater: DesktopUpdater;
  #initialTimer: NodeJS.Timeout | null = null;
  #interval: NodeJS.Timeout | null = null;
  #manualCheck = false;
  #dialogActive = false;

  constructor(options: DesktopUpdateCoordinatorOptions) {
    this.#platform = options.runtime.platform;
    this.#showMessageBox = options.showMessageBox;
    this.#recordEvent = options.recordEvent;
    this.#updater = new DesktopUpdater({
      native: options.native,
      runtime: options.runtime,
      onEvent: (event) => this.#handleEvent(event),
      beforeRestart: options.beforeRestart,
    });
  }

  initialize(): boolean {
    return this.#updater.initialize();
  }

  startAutomaticChecks(): void {
    if (
      this.#updater.snapshot().phase === "disabled" ||
      this.#initialTimer !== null ||
      this.#interval !== null
    ) {
      return;
    }
    this.#initialTimer = setTimeout(() => {
      this.#initialTimer = null;
      void this.#requestCheck(false);
    }, INITIAL_UPDATE_CHECK_DELAY_MS);
    this.#initialTimer.unref();
    this.#interval = setInterval(() => {
      void this.#requestCheck(false);
    }, UPDATE_CHECK_INTERVAL_MS);
    this.#interval.unref();
  }

  stop(): void {
    if (this.#initialTimer !== null) {
      clearTimeout(this.#initialTimer);
      this.#initialTimer = null;
    }
    if (this.#interval !== null) {
      clearInterval(this.#interval);
      this.#interval = null;
    }
  }

  checkManually(): Promise<void> {
    return this.#requestCheck(true);
  }

  async #requestCheck(manual: boolean): Promise<void> {
    const state = this.#updater.snapshot();
    if (state.phase === "disabled") {
      if (manual) {
        await this.#showUnsupported();
      }
      return;
    }
    if (state.phase === "ready") {
      if (manual) {
        await this.#showDownloaded();
      }
      return;
    }
    if (
      state.phase === "checking" ||
      state.phase === "downloading" ||
      state.phase === "restarting"
    ) {
      if (manual) {
        await this.#showMessageBox({
          type: "info",
          buttons: ["OK"],
          defaultId: 0,
          cancelId: 0,
          noLink: true,
          title: "LeapView updates",
          message:
            state.phase === "checking"
              ? "LeapView is already checking for updates."
              : state.phase === "downloading"
                ? "LeapView is downloading an update."
                : "LeapView is restarting to finish the update.",
        });
      }
      return;
    }
    this.#manualCheck = manual;
    if (this.#updater.check() !== "started") {
      this.#manualCheck = false;
    }
  }

  #handleEvent(event: DesktopUpdateEvent): void {
    this.#recordEvent(event);
    switch (event.phase) {
      case "not-available":
        if (this.#manualCheck) {
          this.#manualCheck = false;
          void this.#showMessageBox({
            type: "info",
            buttons: ["OK"],
            defaultId: 0,
            cancelId: 0,
            noLink: true,
            title: "LeapView updates",
            message: "LeapView is up to date.",
          });
        }
        break;
      case "downloaded":
        this.#manualCheck = false;
        void this.#showDownloaded();
        break;
      case "failed":
        if (this.#manualCheck) {
          this.#manualCheck = false;
          void this.#showMessageBox({
            type: "error",
            buttons: ["OK"],
            defaultId: 0,
            cancelId: 0,
            noLink: true,
            title: "LeapView update unavailable",
            message:
              "LeapView could not complete the update check safely.",
            detail:
              "The current installation was not changed. Try again later.",
          });
        }
        break;
      default:
        break;
    }
  }

  async #showDownloaded(): Promise<void> {
    const state = this.#updater.snapshot();
    if (state.phase !== "ready" || this.#dialogActive) {
      return;
    }
    this.#dialogActive = true;
    try {
      const result = await this.#showMessageBox({
        type: "info",
        buttons: ["Later", "Restart now"],
        defaultId: 0,
        cancelId: 0,
        noLink: true,
        title: "LeapView update ready",
        message: `LeapView ${state.version} is ready to install.`,
        detail:
          "Restart LeapView to finish the verified update. Choosing Later keeps the update staged for the next application restart.",
      });
      if (result.response === 1) {
        if (!(await this.#updater.restart())) {
          await this.#showMessageBox({
            type: "error",
            buttons: ["OK"],
            defaultId: 0,
            cancelId: 0,
            noLink: true,
            title: "LeapView could not restart",
            message: "The update remains staged safely.",
            detail: "Close and reopen LeapView to finish the update.",
          });
        }
      } else {
        this.#updater.defer();
      }
    } finally {
      this.#dialogActive = false;
    }
  }

  #showUnsupported(): Promise<MessageBoxReturnValue> {
    return this.#showMessageBox({
      type: "info",
      buttons: ["OK"],
      defaultId: 0,
      cancelId: 0,
      noLink: true,
      title: "LeapView updates",
      message:
        this.#platform === "linux"
          ? "LeapView updates are installed by your system package manager."
          : "Automatic updates are available in installed LeapView release builds.",
      detail:
        this.#platform === "linux"
          ? "Use your normal software updater to install the latest LeapView Desktop package from the LeapView APT repository."
          : "This development or unsupported build will not contact the production update service.",
    });
  }
}
