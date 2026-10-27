// themeFor maps the host's light or dark theme onto Othra's matching daisyUI theme, else follows the system's preference.
export function themeFor(hostTheme: unknown, prefersDark = false): 'othra-light' | 'othra-dark' {
  if (hostTheme === 'dark' || hostTheme === 'light') {
    return hostTheme === 'dark' ? 'othra-dark' : 'othra-light';
  }
  return prefersDark ? 'othra-dark' : 'othra-light';
}

// applyTheme sets Othra's theme on the document from the host context.
export function applyTheme(hostTheme: unknown, root: HTMLElement = document.documentElement) {
  const prefersDark = typeof matchMedia === 'function' && matchMedia('(prefers-color-scheme: dark)').matches;
  root.dataset.theme = themeFor(hostTheme, prefersDark);
}
