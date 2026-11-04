const STORAGE_KEY = 'othra_recent';
const LEGACY_KEY = 'othra_pinned';
const KEEP = 20;

function front(list: string[], slug: string): string[] {
  return [slug, ...list.filter(s => s !== slug)].slice(0, KEEP);
}

function slugs(value: unknown): string[] {
  return Array.isArray(value) ? value.filter((s): s is string => typeof s === 'string') : [];
}

function lists(value: unknown): Record<string, string[]> {
  if (!value || typeof value !== 'object') {
    return {};
  }
  return Object.fromEntries(Object.entries(value).filter(([, v]) => Array.isArray(v)).map(([org, v]) => [org, slugs(v)]));
}

// useRecentStore remembers the orgs and, per org, the projects a person opened last, newest first, in this browser.
export const useRecentStore = defineStore('recent', () => {
  const orgs = ref<string[]>([]);
  const projects = ref<Record<string, string[]>>({});

  function persist() {
    if (!import.meta.client) {
      return;
    }
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify({ orgs: orgs.value, projects: projects.value }));
    } catch {
    }
  }

  function migrate() {
    try {
      const legacy = JSON.parse(localStorage.getItem(LEGACY_KEY) ?? '{}') as { org?: unknown, projects?: Record<string, unknown> };
      orgs.value = typeof legacy.org === 'string' ? [legacy.org] : [];
      projects.value = Object.fromEntries(Object.entries(legacy.projects ?? {}).filter(([, slug]) => typeof slug === 'string').map(([org, slug]) => [org, [slug as string]]));
    } catch {
    }
    persist();
    try {
      localStorage.removeItem(LEGACY_KEY);
    } catch {
    }
  }

  function read() {
    if (!import.meta.client) {
      return;
    }
    try {
      const raw = localStorage.getItem(STORAGE_KEY);
      if (raw === null) {
        migrate();
        return;
      }
      const saved = JSON.parse(raw) as { orgs?: unknown, projects?: unknown };
      orgs.value = slugs(saved.orgs);
      projects.value = lists(saved.projects);
    } catch {
    }
  }

  function visit(org: string, project?: string) {
    orgs.value = front(orgs.value, org);
    const kept = Object.fromEntries(Object.entries(projects.value).filter(([o]) => orgs.value.includes(o)));
    if (project) {
      kept[org] = front(kept[org] ?? [], project);
    }
    projects.value = kept;
    persist();
  }

  function lastOrg() {
    return orgs.value[0] ?? '';
  }

  function lastProject(org: string) {
    return projects.value[org]?.[0] ?? '';
  }

  function byRecency<T>(items: T[], slug: (item: T) => string, order: string[]): T[] {
    const rank = (item: T) => {
      const i = order.indexOf(slug(item));
      return i === -1 ? order.length : i;
    };
    return [...items].sort((a, b) => rank(a) - rank(b));
  }

  read();

  return { orgs, projects, visit, lastOrg, lastProject, byRecency };
});
