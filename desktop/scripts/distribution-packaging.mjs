export function requirePackagedDistribution(value) {
  if (value === "preview" || value === "stable") {
    return value;
  }
  throw new Error(
    "LEAPVIEW_DESKTOP_DISTRIBUTION must be explicitly set to preview or stable",
  );
}
