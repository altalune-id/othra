import { describe, expect, it } from 'vitest';

import { bestTextColor, contrastRatio, formatRatio, isLowContrast, normalizeHex } from './contrast';

describe('normalizeHex', () => {
  it('accepts #rrggbb and #rgb in any case and returns lowercase #rrggbb', () => {
    expect(normalizeHex('#4F46E5')).toBe('#4f46e5');
    expect(normalizeHex(' #AbC ')).toBe('#aabbcc');
  });

  it('refuses anything that is not a hex colour', () => {
    expect(normalizeHex('')).toBe('');
    expect(normalizeHex('4f46e5')).toBe('');
    expect(normalizeHex('#4f46e')).toBe('');
    expect(normalizeHex('#ggg')).toBe('');
  });
});

describe('contrastRatio', () => {
  it('is 21 for black on white and 1 for a colour on itself', () => {
    expect(contrastRatio('#000000', '#ffffff')).toBeCloseTo(21, 5);
    expect(contrastRatio('#ffffff', '#000000')).toBeCloseTo(21, 5);
    expect(contrastRatio('#4f46e5', '#4f46e5')).toBeCloseTo(1, 5);
  });

  it('matches a known pair', () => {
    expect(contrastRatio('#4f46e5', '#ffffff')).toBeCloseTo(6.29, 1);
  });
});

describe('bestTextColor', () => {
  it('picks white on dark colours and near-black on light ones', () => {
    expect(bestTextColor('#4f46e5')).toBe('#ffffff');
    expect(bestTextColor('#fde047')).toBe('#111827');
  });
});

describe('isLowContrast', () => {
  it('warns only when even the best text colour stays under 4.5:1', () => {
    expect(isLowContrast('#4f46e5')).toBe(false);
    expect(isLowContrast('#6b7280')).toBe(false);
    expect(isLowContrast('#777777')).toBe(true);
  });
});

describe('formatRatio', () => {
  it('rounds to one decimal', () => {
    expect(formatRatio(6.2891)).toBe('6.3');
    expect(formatRatio(21)).toBe('21.0');
  });
});
