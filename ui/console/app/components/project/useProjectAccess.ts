import type { Caller } from '~/composables/useCan';
import { can, useCan } from '~/composables/useCan';

// canEditProject mirrors the server's project:write check; with nobody signed in the server decides.
export function canEditProject(me: Caller | null): boolean {
  return can(me, 'project:write');
}

export function useCanEditProject() {
  return useCan('project:write');
}
