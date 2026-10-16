import { useAuthStore } from '~/stores/auth';
import { useOrgsStore } from '~/stores/orgs';

// A person works in one org at a time; opening another org switches the session to it first. Operators reach every org without it.
export function useActiveOrg() {
  const auth = useAuthStore();
  const orgs = useOrgsStore();

  async function enter(slug: string): Promise<void> {
    await Promise.all([auth.load(), orgs.ensureLoaded()]);
    const me = auth.me;
    const org = orgs.bySlug(slug);
    if (!me || !org || me.source !== 'local' || me.operator || me.activeOrgSlug === slug) {
      return;
    }
    await auth.setActiveOrg(org.id);
  }

  return { enter };
}
