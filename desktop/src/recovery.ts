export type RecoveryAttemptResult = "success" | "retry" | "stop";

export interface RecoveryCoordinatorOptions {
  delaysMs?: readonly number[];
  random?: () => number;
  setTimer?: (run: () => void, delayMs: number) => unknown;
  clearTimer?: (timer: unknown) => void;
  onExhausted?: () => void;
}

const DEFAULT_DELAYS_MS = [1_000, 3_000, 8_000] as const;
const JITTER_RATIO = 0.2;

export class RollingRecoveryBudget {
  readonly #maximumAttempts: number;
  readonly #windowMs: number;
  #attempts: number[] = [];

  constructor(maximumAttempts: number, windowMs: number) {
    if (
      !Number.isSafeInteger(maximumAttempts) ||
      maximumAttempts < 1 ||
      maximumAttempts > 10 ||
      !Number.isSafeInteger(windowMs) ||
      windowMs < 1 ||
      windowMs > 86_400_000
    ) {
      throw new TypeError("rolling recovery budget is invalid");
    }
    this.#maximumAttempts = maximumAttempts;
    this.#windowMs = windowMs;
  }

  consume(now = performance.now()): boolean {
    if (!Number.isFinite(now) || now < 0) {
      throw new TypeError("recovery budget time is invalid");
    }
    const previous = this.#attempts.at(-1);
    if (previous !== undefined && now < previous) {
      this.#attempts = [];
    }
    const threshold = now - this.#windowMs;
    this.#attempts = this.#attempts.filter(
      (attempt) => attempt >= threshold,
    );
    if (this.#attempts.length >= this.#maximumAttempts) {
      return false;
    }
    this.#attempts.push(now);
    return true;
  }

  reset(): void {
    this.#attempts = [];
  }
}

export class BoundedRecoveryCoordinator {
  readonly #attempt: (
    signal: AbortSignal,
  ) => Promise<RecoveryAttemptResult>;
  readonly #delaysMs: readonly number[];
  readonly #random: () => number;
  readonly #setTimer: (run: () => void, delayMs: number) => unknown;
  readonly #clearTimer: (timer: unknown) => void;
  readonly #onExhausted: () => void;
  #available = true;
  #pending = false;
  #inFlight = false;
  #attemptIndex = 0;
  #timer: unknown;
  #controller: AbortController | undefined;
  #generation = 0;

  constructor(
    attempt: (signal: AbortSignal) => Promise<RecoveryAttemptResult>,
    options: RecoveryCoordinatorOptions = {},
  ) {
    this.#attempt = attempt;
    this.#delaysMs = options.delaysMs ?? DEFAULT_DELAYS_MS;
    this.#random = options.random ?? Math.random;
    this.#setTimer = options.setTimer ??
      ((run, delayMs) => setTimeout(run, delayMs));
    this.#clearTimer = options.clearTimer ??
      ((timer) => clearTimeout(timer as NodeJS.Timeout));
    this.#onExhausted = options.onExhausted ?? (() => undefined);
    if (
      this.#delaysMs.length === 0 ||
      this.#delaysMs.length > 10 ||
      this.#delaysMs.some(
        (delay) =>
          !Number.isSafeInteger(delay) ||
          delay < 0 ||
          delay > 60_000,
      )
    ) {
      throw new TypeError("recovery delays are invalid");
    }
  }

  get pending(): boolean {
    return this.#pending;
  }

  request(): void {
    if (this.#pending) {
      return;
    }
    this.#pending = true;
    this.#attemptIndex = 0;
    this.#schedule(this.#delaysMs[0]!);
  }

  setAvailable(available: boolean): void {
    if (this.#available === available) {
      return;
    }
    this.#available = available;
    if (!available) {
      this.#clearScheduledTimer();
      this.#controller?.abort();
      return;
    }
    if (this.#pending && !this.#inFlight) {
      this.#schedule(0);
    }
  }

  cancel(): void {
    this.#generation += 1;
    this.#controller?.abort();
    this.#controller = undefined;
    this.#pending = false;
    this.#attemptIndex = 0;
    this.#clearScheduledTimer();
  }

  #schedule(baseDelayMs: number): void {
    if (!this.#pending || !this.#available || this.#timer !== undefined) {
      return;
    }
    const random = this.#random();
    const boundedRandom =
      Number.isFinite(random) && random >= 0 && random <= 1
        ? random
        : 0.5;
    const multiplier =
      1 - JITTER_RATIO + boundedRandom * JITTER_RATIO * 2;
    const delayMs = Math.round(baseDelayMs * multiplier);
    const generation = this.#generation;
    this.#timer = this.#setTimer(() => {
      this.#timer = undefined;
      void this.#run(generation);
    }, delayMs);
  }

  async #run(generation: number): Promise<void> {
    if (
      generation !== this.#generation ||
      !this.#pending ||
      !this.#available ||
      this.#inFlight
    ) {
      return;
    }
    this.#inFlight = true;
    const controller = new AbortController();
    this.#controller = controller;
    let result: RecoveryAttemptResult;
    try {
      result = await this.#attempt(controller.signal);
    } catch {
      result = "retry";
    } finally {
      this.#inFlight = false;
      if (this.#controller === controller) {
        this.#controller = undefined;
      }
    }
    if (generation !== this.#generation || !this.#pending) {
      return;
    }
    if (controller.signal.aborted) {
      this.#schedule(0);
      return;
    }
    if (result !== "retry") {
      this.cancel();
      return;
    }
    this.#attemptIndex += 1;
    if (this.#attemptIndex >= this.#delaysMs.length) {
      this.cancel();
      this.#onExhausted();
      return;
    }
    this.#schedule(this.#delaysMs[this.#attemptIndex]!);
  }

  #clearScheduledTimer(): void {
    if (this.#timer === undefined) {
      return;
    }
    this.#clearTimer(this.#timer);
    this.#timer = undefined;
  }
}
