import { describe, expect, it } from 'vitest';
import { t } from './copy';

describe('copy', () => {
  it('reads the shared base copy and fills params', () => {
    expect(t('rs.state_verified')).toBe('Verified');
    expect(t('rs.reason_status', { status: 503 })).toBe('The metadata URL returned HTTP 503.');
    expect(t('rs.reason_status')).toBe('The metadata URL returned HTTP {status}.');
    expect(t('no.such.key')).toBe('no.such.key');
  });
});
