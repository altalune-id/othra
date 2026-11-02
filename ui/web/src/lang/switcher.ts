const YEAR = 60 * 60 * 24 * 365;

export function langCookie(locale: string, path: string, secure: boolean): string {
  return `lang=${encodeURIComponent(locale)}; Path=${path || '/'}; Max-Age=${YEAR}; SameSite=Lax${secure ? '; Secure' : ''}`;
}

export function enhance(root: ParentNode, loc: Pick<Location, 'protocol' | 'reload'>): void {
  const select = root.querySelector<HTMLSelectElement>('[data-lang-select]');
  if (!select) {
    return;
  }
  select.hidden = false;
  select.addEventListener('change', () => {
    document.cookie = langCookie(select.value, select.dataset.path ?? '/', loc.protocol === 'https:');
    loc.reload();
  });
}
