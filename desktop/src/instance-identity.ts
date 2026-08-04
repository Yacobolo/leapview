const instanceIDPattern = /^(?:lvinst_[A-Za-z0-9_-]{32}|instance_[0-9a-f]{32})$/u;

export function isValidInstanceID(value: string): boolean {
  return instanceIDPattern.test(value);
}
