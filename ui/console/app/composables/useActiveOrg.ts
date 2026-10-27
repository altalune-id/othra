import { isPerson } from '~/lib/auth/person';
import { useAuthStore } from '~/stores/auth';
import { useOrgsStore } from '~/stores/orgs';

// A person works in one org at a time; opening another org switches the session to it first, so every call runs in that org.
export function useActiveOrg() {
  const auth = useAuthStore();
  const orgs = useOrgsStore();

  async function enter(slug: string): Promise<void> {
    await Promise.all([auth.load(), orgs.ensureLoaded()]);
    const me = auth.me;
    const org = orgs.bySlug(slug);
    if (!me || !org || !isPerson(me) || me.activeOrgSlug === slug) {
      return;
    }
    if (!me.operator) {
      await auth.setActiveOrg(org.id);
      return;
    }
    try {
      await auth.setActiveOrg(org.id);
    } catch {
      // NOTE: an operator may open an org they don't belong to; the switch is refused there and they keep their reach.
    }
  }

  return { enter };
}
