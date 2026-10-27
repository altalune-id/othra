export type ThemeMode = 'light' | 'dark';

export interface ThemeFamily {
  id: string;
  label: string;
  themes: Partial<Record<ThemeMode, string>>;
  default: ThemeMode;
}

// NOTE: keep in sync with the @plugin "daisyui/theme" blocks in ui/theme.css.
export const themeFamilies: ThemeFamily[] = [
  { id: 'othra', label: 'Othra', default: 'light', themes: { light: 'othra-light', dark: 'othra-dark' } },
  { id: 'bumblebee', label: 'Bumblebee', default: 'light', themes: { light: 'bumblebee-light', dark: 'bumblebee-dark' } },
  { id: 'retro', label: 'Retro', default: 'light', themes: { light: 'retro-light', dark: 'retro-dark' } },
  { id: 'pastel', label: 'Pastel', default: 'light', themes: { light: 'pastel-light', dark: 'pastel-dark' } },
  { id: 'caramellatte', label: 'Caramellatte', default: 'light', themes: { light: 'caramellatte-light', dark: 'caramellatte-dark' } },
  { id: 'coffee', label: 'Coffee', default: 'dark', themes: { dark: 'coffee-dark' } },
  { id: 'luxury', label: 'Luxury', default: 'dark', themes: { dark: 'luxury-dark' } },
  { id: 'night', label: 'Night', default: 'dark', themes: { dark: 'night-dark' } },
  { id: 'halloween', label: 'Halloween', default: 'dark', themes: { dark: 'halloween-dark' } },
];

const FALLBACK: ThemeFamily = themeFamilies[0]!;

function themeName(family: ThemeFamily, mode: ThemeMode): string {
  return family.themes[mode] ?? family.themes[family.default]!;
}

// useTheme keeps the app's theme in one state, stored under the app config's theme.storageKey and following the OS while theme.followSystem is set.
export function useTheme() {
  const settings = useAppConfig().theme;
  const storageKey = computed(() => settings.storageKey);
  const followSystem = computed(() => settings.followSystem);
  const familyId = useState<string>('theme.family', () => FALLBACK.id);
  const mode = useState<ThemeMode>('theme.mode', () => FALLBACK.default);
  const picked = useState<boolean>('theme.picked', () => false);

  const family = computed<ThemeFamily>(() => themeFamilies.find(f => f.id === familyId.value) ?? FALLBACK);
  const supportsMode = computed(() => Object.keys(family.value.themes).length > 1);

  function paint() {
    if (import.meta.client) {
      document.documentElement.setAttribute('data-theme', themeName(family.value, mode.value));
    }
  }

  function persist() {
    picked.value = true;
    try {
      localStorage.setItem(storageKey.value, JSON.stringify({ family: familyId.value, mode: mode.value }));
    } catch {
    }
  }

  function apply() {
    paint();
    if (import.meta.client) {
      persist();
    }
  }

  function setFamily(id: string) {
    const next = themeFamilies.find(f => f.id === id);
    if (!next) {
      return;
    }
    familyId.value = next.id;
    mode.value = next.default;
    apply();
  }

  function setMode(next: ThemeMode) {
    if (!family.value.themes[next]) {
      return;
    }
    mode.value = next;
    apply();
  }

  function toggle() {
    if (!supportsMode.value) {
      return;
    }
    setMode(mode.value === 'dark' ? 'light' : 'dark');
  }

  function init() {
    if (!import.meta.client) {
      return;
    }
    let restored = false;
    try {
      const saved = JSON.parse(localStorage.getItem(storageKey.value) ?? '{}') as { family?: string, mode?: ThemeMode };
      const next = themeFamilies.find(f => f.id === saved.family);
      if (next) {
        familyId.value = next.id;
        mode.value = saved.mode && next.themes[saved.mode] ? saved.mode : next.default;
        restored = true;
      }
    } catch {
    }
    picked.value = restored;
    if (!followSystem.value) {
      apply();
      return;
    }
    const media = globalThis.matchMedia?.('(prefers-color-scheme: dark)');
    if (!restored && media?.matches) {
      mode.value = 'dark';
    }
    paint();
    media?.addEventListener?.('change', (event: MediaQueryListEvent) => {
      if (!picked.value) {
        mode.value = event.matches ? 'dark' : 'light';
        paint();
      }
    });
  }

  return { family, mode, supportsMode, families: themeFamilies, setFamily, setMode, toggle, init };
}
