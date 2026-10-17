const darkText = '#111827';
const lightText = '#ffffff';
const minimum = 4.5;

// normalizeHex returns a #rgb or #rrggbb colour as lowercase #rrggbb, or '' when it is not one.
export function normalizeHex(value: string): string {
  const v = value.trim().toLowerCase();
  if (/^#[0-9a-f]{6}$/.test(v)) {
    return v;
  }
  if (/^#[0-9a-f]{3}$/.test(v)) {
    return `#${[...v.slice(1)].map(c => c + c).join('')}`;
  }
  return '';
}

function channel(hex: string, at: number): number {
  const c = Number.parseInt(hex.slice(at, at + 2), 16) / 255;
  return c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
}

function luminance(hex: string): number {
  return 0.2126 * channel(hex, 1) + 0.7152 * channel(hex, 3) + 0.0722 * channel(hex, 5);
}

// contrastRatio is the WCAG 2 contrast ratio of two #rrggbb colours.
export function contrastRatio(a: string, b: string): number {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x) as [number, number];
  return (hi + 0.05) / (lo + 0.05);
}

// bestTextColor picks the button text colour, white or near-black, that reads best on background.
export function bestTextColor(background: string): string {
  return contrastRatio(background, lightText) >= contrastRatio(background, darkText) ? lightText : darkText;
}

// isLowContrast reports whether even the best text colour stays under WCAG AA's 4.5:1.
export function isLowContrast(background: string): boolean {
  return contrastRatio(background, bestTextColor(background)) < minimum;
}

export function formatRatio(ratio: number): string {
  return ratio.toFixed(1);
}
