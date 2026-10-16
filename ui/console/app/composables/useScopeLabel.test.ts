import { describe, expect, it, vi } from 'vitest';

import { useScopeLabel } from './useScopeLabel';

describe('useScopeLabel', () => {
  it('labels a known scope', () => {
    expect(useScopeLabel()('webhook:read')).toBe('scope.webhook_read');
  });

  it('shows the raw name of an unknown scope', () => {
    expect(useScopeLabel()('mystery:do')).toBe('mystery:do');
  });
});

describe('useScopeLabel outside a component setup', () => {
  it('never calls useI18n', () => {
    vi.stubGlobal('useI18n', () => {
      throw new SyntaxError('26');
    });
    expect(useScopeLabel()('webhook:read')).toBe('scope.webhook_read');
  });
});
