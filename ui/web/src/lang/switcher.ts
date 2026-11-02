const YEAR = 60 * 60 * 24 * 365;

export function langCookie(name: string, locale: string, secure: boolean): string {
  return `${name}=${encodeURIComponent(locale)}; Path=/; Max-Age=${YEAR}; SameSite=Lax${secure ? '; Secure' : ''}`;
}

export function enhance(root: ParentNode, loc: Pick<Location, 'protocol' | 'reload'>): void {
  const select = root.querySelector<HTMLSelectElement>('[data-lang-select]');
  const name = select?.dataset.cookie;
  if (!select || !name) {
    return;
  }
  select.hidden = false;
  select.addEventListener('change', () => {
    document.cookie = langCookie(name, select.value, loc.protocol === 'https:');
    loc.reload();
  });
}
