import { describe, expect, it } from 'vitest';

import { device } from './device';

describe('device', () => {
  it.each([
    ['Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36', 'Chrome', 'macOS'],
    ['Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36 Edg/141.0.0.0', 'Edge', 'Windows'],
    ['Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1', 'Safari', 'iOS'],
    ['Mozilla/5.0 (X11; Linux x86_64; rv:144.0) Gecko/20100101 Firefox/144.0', 'Firefox', 'Linux'],
    ['Mozilla/5.0 (Linux; Android 15) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0 Mobile Safari/537.36 OPR/90.0', 'Opera', 'Android'],
    ['Mozilla/5.0 (X11; CrOS x86_64 14541.0.0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36', 'Chrome', 'ChromeOS'],
    ['curl/8.9.1', '', ''],
    ['', '', ''],
  ])('%s', (ua, browser, os) => {
    expect(device(ua)).toEqual({ browser, os });
  });
});
