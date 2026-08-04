export function packagedProofOrigin(raw: string): string {
  const parsed = new URL(raw);
  if (
    parsed.protocol !== "http:" ||
    (parsed.hostname !== "127.0.0.1" && parsed.hostname !== "::1") ||
    parsed.username !== "" || parsed.password !== "" ||
    parsed.pathname !== "/" || parsed.search !== "" || parsed.hash !== "" ||
    parsed.port === ""
  ) {
    throw new Error("packaged proof origin must be an exact loopback HTTP origin");
  }
  return parsed.origin;
}
