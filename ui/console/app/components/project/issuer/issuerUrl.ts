// splitIssuerUrl cuts an issuer URL after its last /p/, so it can wrap between the base and the handle.
export function splitIssuerUrl(url: string): { base: string, handle: string } {
  const at = url.lastIndexOf('/p/');
  if (at < 0) {
    return { base: url, handle: '' };
  }
  return { base: url.slice(0, at + 3), handle: url.slice(at + 3) };
}
