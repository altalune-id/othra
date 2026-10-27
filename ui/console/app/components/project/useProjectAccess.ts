import type { MaybeRefOrGetter } from 'vue';
import type { Caller } from '~/composables/useCan';
import { toValue } from 'vue';
import { can, useCan } from '~/composables/useCan';
import { useAuthStore } from '~/stores/auth';

// canEditProject mirrors the server's project:write check; with nobody signed in the server decides.
export function canEditProject(me: Caller | null): boolean {
  return can(me, 'project:write');
}

export function useCanEditProject() {
  return useCan('project:write');
}

export interface ProjectGuard {
  canWrite: boolean;
  canConfigure: boolean;
  system: boolean;
}

// projectGuard says what a person may change in project; the console project keeps only its branding, terms, keys and upstream IdPs writable.
export function projectGuard(me: Caller | null, project?: { system?: boolean }): ProjectGuard {
  const canWrite = canEditProject(me);
  const system = !!project?.system;
  return { canWrite, canConfigure: canWrite && !system, system };
}

export function useProjectGuard(project: MaybeRefOrGetter<{ system?: boolean } | undefined>) {
  const auth = useAuthStore();
  return computed(() => projectGuard(auth.me, toValue(project)));
}
