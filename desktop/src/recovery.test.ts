import { describe, expect, test } from "bun:test";

import {
  BoundedRecoveryCoordinator,
  RollingRecoveryBudget,
  type RecoveryAttemptResult,
} from "./recovery.js";

interface ScheduledTask {
  delay: number;
  run: () => void;
}

function harness(
  results: RecoveryAttemptResult[],
  onExhausted: () => void = () => undefined,
) {
  const scheduled: ScheduledTask[] = [];
  let attempts = 0;
  const coordinator = new BoundedRecoveryCoordinator(
    async () => {
      attempts += 1;
      return results.shift() ?? "stop";
    },
    {
      delaysMs: [1_000, 3_000, 8_000],
      random: () => 0.5,
      setTimer: (run, delay) => {
        const task = { delay, run };
        scheduled.push(task);
        return task;
      },
      clearTimer: (timer) => {
        const index = scheduled.indexOf(timer as ScheduledTask);
        if (index >= 0) {
          scheduled.splice(index, 1);
        }
      },
      onExhausted,
    },
  );
  return {
    coordinator,
    scheduled,
    attempts: () => attempts,
    async runNext() {
      const task = scheduled.shift();
      expect(task).toBeDefined();
      task!.run();
      await Promise.resolve();
      await Promise.resolve();
    },
  };
}

describe("BoundedRecoveryCoordinator", () => {
  test("coalesces failures and retries with a bounded deterministic schedule", async () => {
    const recovery = harness(["retry", "retry", "success"]);

    recovery.coordinator.request();
    recovery.coordinator.request();
    expect(recovery.scheduled.map((task) => task.delay)).toEqual([1_000]);

    await recovery.runNext();
    expect(recovery.attempts()).toBe(1);
    expect(recovery.scheduled.map((task) => task.delay)).toEqual([3_000]);

    await recovery.runNext();
    expect(recovery.attempts()).toBe(2);
    expect(recovery.scheduled.map((task) => task.delay)).toEqual([8_000]);

    await recovery.runNext();
    expect(recovery.attempts()).toBe(3);
    expect(recovery.scheduled).toEqual([]);
    expect(recovery.coordinator.pending).toBe(false);
  });

  test("pauses while unavailable without consuming an attempt", async () => {
    const recovery = harness(["success"]);
    recovery.coordinator.setAvailable(false);
    recovery.coordinator.request();

    expect(recovery.scheduled).toEqual([]);
    expect(recovery.attempts()).toBe(0);
    expect(recovery.coordinator.pending).toBe(true);

    recovery.coordinator.setAvailable(true);
    expect(recovery.scheduled.map((task) => task.delay)).toEqual([0]);
    await recovery.runNext();
    expect(recovery.attempts()).toBe(1);
  });

  test("stops after the configured attempt budget and cancels cleanly", async () => {
    let exhausted = 0;
    const recovery = harness(
      ["retry", "retry", "retry", "success"],
      () => {
        exhausted += 1;
      },
    );
    recovery.coordinator.request();
    await recovery.runNext();
    await recovery.runNext();
    await recovery.runNext();

    expect(recovery.attempts()).toBe(3);
    expect(recovery.scheduled).toEqual([]);
    expect(recovery.coordinator.pending).toBe(false);
    expect(exhausted).toBe(1);

    recovery.coordinator.request();
    expect(recovery.scheduled).toHaveLength(1);
    recovery.coordinator.cancel();
    expect(recovery.scheduled).toEqual([]);
    expect(recovery.coordinator.pending).toBe(false);
  });

  test("aborts an in-flight attempt when lifecycle availability is lost", async () => {
    const scheduled: ScheduledTask[] = [];
    let attempts = 0;
    let observedAbort = false;
    const coordinator = new BoundedRecoveryCoordinator(
      (signal) =>
        new Promise<RecoveryAttemptResult>((resolve) => {
          attempts += 1;
          signal.addEventListener(
            "abort",
            () => {
              observedAbort = true;
              resolve("retry");
            },
            { once: true },
          );
        }),
      {
        delaysMs: [1_000, 3_000],
        random: () => 0.5,
        setTimer: (run, delay) => {
          const task = { delay, run };
          scheduled.push(task);
          return task;
        },
        clearTimer: (timer) => {
          const index = scheduled.indexOf(timer as ScheduledTask);
          if (index >= 0) {
            scheduled.splice(index, 1);
          }
        },
      },
    );

    coordinator.request();
    const first = scheduled.shift();
    first!.run();
    await Promise.resolve();
    coordinator.setAvailable(false);
    await Promise.resolve();
    await Promise.resolve();

    expect(observedAbort).toBe(true);
    expect(attempts).toBe(1);
    expect(coordinator.pending).toBe(true);
    expect(scheduled).toEqual([]);

    coordinator.setAvailable(true);
    expect(scheduled.map((task) => task.delay)).toEqual([0]);
    coordinator.cancel();
  });
});

describe("RollingRecoveryBudget", () => {
  test("bounds repeated renderer recreation inside a rolling window", () => {
    const budget = new RollingRecoveryBudget(2, 60_000);

    expect(budget.consume(1_000)).toBe(true);
    expect(budget.consume(2_000)).toBe(true);
    expect(budget.consume(3_000)).toBe(false);
    expect(budget.consume(61_001)).toBe(true);
    budget.reset();
    expect(budget.consume(61_002)).toBe(true);
  });
});
