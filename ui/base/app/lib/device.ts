export interface Device {
  browser: string;
  os: string;
}

const browsers: [RegExp, string][] = [
  [/\bEdg(?:e|A|iOS)?\//, 'Edge'],
  [/\bOPR\/|\bOpera\b/, 'Opera'],
  [/\bFirefox\/|\bFxiOS\//, 'Firefox'],
  [/\bChrome\/|\bCriOS\//, 'Chrome'],
  [/\bVersion\/[\d.].*\bSafari\//, 'Safari'],
];

const systems: [RegExp, string][] = [
  [/\biPhone|\biPad|\biPod/, 'iOS'],
  [/\bAndroid\b/, 'Android'],
  [/\bCrOS\b/, 'ChromeOS'],
  [/\bWindows\b/, 'Windows'],
  [/\bMac OS X\b|\bMacintosh\b/, 'macOS'],
  [/\bLinux\b/, 'Linux'],
];

function first(ua: string, table: [RegExp, string][]): string {
  return table.find(([re]) => re.test(ua))?.[1] ?? '';
}

// device names the browser and system a user agent claims; either is empty when unknown, and the claim is never trusted for security.
export function device(userAgent: string): Device {
  return { browser: first(userAgent, browsers), os: first(userAgent, systems) };
}
