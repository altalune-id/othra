const STORAGE_KEY = 'othra_recent';
const LEGACY_KEY = 'othra_pinned';
const KEEP = 20;

interface Saved {
  orgs?: string[];
  projects?: Record<string, string[]>;
}

function front(list: string[], slug: string): string[] {
  return [slug, ...list.filter(s => s !== slug)].slice(0, KEEP);
}

// useRecentStore remembers the orgs and, per org, the projects a person opened last, newest first, in this browser.
export const useRecentStore = defineStore('recent', () => {
  const orgs = ref<string[]>([]);
  const projects = ref<Record<string, string[]>>({});

  function persist() {
    if (!import.meta.client) {
      return;
    }
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ orgs: orgs.value, projects: projects.value }));
  }

  function migrate() {
    const legacy = JSON.parse(localStorage.getItem(LEGACY_KEY) ?? '{}') as { org?: string, projects?: Record<string, string> };
    orgs.value = legacy.org ? [legacy.org] : [];
    projects.value = Object.fromEntries(Object.entries(legacy.projects ?? {}).map(([org, slug]) => [org, [slug]]));
    persist();
    localStorage.removeItem(LEGACY_KEY);
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
      const saved = JSON.parse(raw) as Saved;
      orgs.value = Array.isArray(saved.orgs) ? saved.orgs : [];
      projects.value = saved.projects ?? {};
    } catch {
    }
  }

  function visit(org: string, project?: string) {
    orgs.value = front(orgs.value, org);
    if (project) {
      projects.value = { ...projects.value, [org]: front(projects.value[org] ?? [], project) };
    }
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
