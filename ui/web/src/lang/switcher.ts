const YEAR = 60 * 60 * 24 * 365;

export function langCookie(locale: string, secure: boolean): string {
  return `othra_lang=${encodeURIComponent(locale)}; Path=/; Max-Age=${YEAR}; SameSite=Lax${secure ? '; Secure' : ''}`;
}

export function enhance(root: ParentNode, loc: Pick<Location, 'protocol' | 'reload'>): void {
  const select = root.querySelector<HTMLSelectElement>('[data-lang-select]');
  if (!select) {
    return;
  }
  select.hidden = false;
  select.addEventListener('change', () => {
    document.cookie = langCookie(select.value, loc.protocol === 'https:');
    loc.reload();
  });
}
