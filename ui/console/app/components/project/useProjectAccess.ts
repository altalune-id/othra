import { grants } from '~/components/apikey/personal/grants';
import { useAuthStore } from '~/stores/auth';

// canEditProject mirrors the server's project:write check; with nobody signed in the server decides.
export function canEditProject(me: { operator?: boolean, permissions?: string[] } | null): boolean {
  return !me || !!me.operator || grants(me.permissions ?? [], 'project:write');
}

export function useCanEditProject() {
  const auth = useAuthStore();
  return computed(() => canEditProject(auth.me));
}
