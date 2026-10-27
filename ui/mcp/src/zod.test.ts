import { describe, expect, it } from 'vitest';
import { config } from 'zod';
import './zod';

describe('zod', () => {
  it('runs jitless, so the app never calls Function() under the host CSP', () => {
    expect(config().jitless).toBe(true);
  });
});
