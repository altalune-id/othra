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
  it('floors to one decimal so the label never crosses a threshold the verdict disagrees with', () => {
    expect(formatRatio(6.2891)).toBe('6.2');
    expect(formatRatio(21)).toBe('21.0');
    expect(formatRatio(4.48)).toBe('4.4');
    expect(formatRatio(4.4999)).toBe('4.4');
    expect(formatRatio(4.5)).toBe('4.5');
    expect(formatRatio(2.99)).toBe('2.9');
    expect(formatRatio(3)).toBe('3.0');
  });

  it('labels a low-contrast colour below 4.5', () => {
    const ratio = contrastRatio('#777777', bestTextColor('#777777'));
    expect(isLowContrast('#777777')).toBe(true);
    expect(Number(formatRatio(ratio))).toBeLessThan(4.5);
  });
});
