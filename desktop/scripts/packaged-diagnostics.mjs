const processGoneReasons = new Set([
  "clean-exit",
  "abnormal-exit",
  "killed",
  "crashed",
  "oom",
  "launch-failed",
  "integrity-failure",
  "memory-eviction",
]);

const updatePhases = new Set([
  "checking",
  "available",
  "not-available",
  "downloaded",
  "deferred",
  "restart-requested",
  "failed",
]);

export function verifyPackagedDiagnosticEvent(event) {
  if (typeof event !== "object" || event === null || Array.isArray(event)) {
    throw new Error("packaged diagnostic journal contains an invalid event");
  }
  if (
    typeof event.at !== "string" ||
    new Date(event.at).toISOString() !== event.at
  ) {
    throw new Error(
      "packaged diagnostic journal contains an invalid timestamp",
    );
  }
  if (event.kind === "startup") {
    if (
      Object.keys(event).sort().join(",") !== "at,kind,packaged" ||
      event.packaged !== true
    ) {
      throw new Error(
        "packaged diagnostic journal contains invalid startup data",
      );
    }
    return;
  }
  if (event.kind === "policy") {
    if (
      Object.keys(event).sort().join(",") !==
        "at,diagnostics,kind,mode,userInstances" ||
      !["open", "managed", "locked"].includes(event.mode) ||
      !["allowed", "restricted"].includes(event.userInstances) ||
      !["enabled", "disabled"].includes(event.diagnostics)
    ) {
      throw new Error(
        "packaged diagnostic journal contains invalid policy data",
      );
    }
    return;
  }
  if (event.kind === "update") {
    if (
      Object.keys(event).sort().join(",") !== "at,kind,phase" ||
      !updatePhases.has(event.phase)
    ) {
      throw new Error(
        "packaged diagnostic journal contains invalid update data",
      );
    }
    return;
  }
  if (event.kind === "render-process-gone") {
    if (
      Object.keys(event).sort().join(",") !== "at,kind,reason,surface" ||
      !["trusted-shell", "unknown"].includes(event.surface) ||
      !processGoneReasons.has(event.reason)
    ) {
      throw new Error(
        "packaged diagnostic journal contains invalid renderer data",
      );
    }
    return;
  }
  if (event.kind === "child-process-gone") {
    if (
      Object.keys(event).sort().join(",") !== "at,kind,processType,reason" ||
      ![
        "utility",
        "zygote",
        "sandbox-helper",
        "gpu",
        "pepper-plugin",
        "pepper-plugin-broker",
        "unknown",
      ].includes(event.processType) ||
      !processGoneReasons.has(event.reason)
    ) {
      throw new Error(
        "packaged diagnostic journal contains invalid child-process data",
      );
    }
    return;
  }
  throw new Error(
    "packaged diagnostic journal contains an unexpected startup event",
  );
}
