import type { NavItem } from '#base/components/ui/types';

type Translate = (key: string) => string;

export const screens = [
  { key: 'profile', path: '/', icon: 'lucide:user-round' },
  { key: 'sign_in', path: '/sign-in', icon: 'lucide:log-in' },
  { key: 'sessions', path: '/sessions', icon: 'lucide:monitor-smartphone' },
  { key: 'apps', path: '/apps', icon: 'lucide:blocks' },
  { key: 'tokens', path: '/tokens', icon: 'lucide:key-round' },
] as const;

// navItems marks the screen a route path belongs to; nested paths such as /tokens/new keep their parent active.
export function navItems(path: string, t: Translate): NavItem[] {
  return screens.map(s => ({
    key: s.key,
    label: t(`nav.${s.key}`),
    icon: s.icon,
    active: s.path === '/' ? path === '/' : path === s.path || path.startsWith(`${s.path}/`),
  }));
}

// screenPath is the route path of a nav key.
export function screenPath(key: string): string {
  return screens.find(s => s.key === key)?.path ?? '/';
}
