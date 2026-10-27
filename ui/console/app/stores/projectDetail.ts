import type { ShownError } from '#base/lib/apperror/message';
import type { Project } from '#gen/project/v1/project_pb';
import { useErrorMessage } from '#base/composables/useErrorMessage';

// useProjectDetailStore holds each project's full settings from GetProject; the projects store keeps only the list.
export const useProjectDetailStore = defineStore('projectDetail', () => {
  const byId = ref<Record<string, Project>>({});
  const loading = ref(false);
  const error = ref<ShownError | null>(null);

  const { $projectClient: client } = useNuxtApp();
  const errorMessage = useErrorMessage();
  const inflight = new Map<string, Promise<void>>();

  function get(id: string): Project | undefined {
    return byId.value[id];
  }

  function replace(project: Project) {
    byId.value = { ...byId.value, [project.id]: project };
  }

  async function load(id: string) {
    loading.value = true;
    error.value = null;
    try {
      const res = await client.getProject({ id });
      if (res.project) {
        replace(res.project);
      }
    } catch (e) {
      error.value = errorMessage(e, 'error.load_project');
    } finally {
      loading.value = false;
    }
  }

  function ensureLoaded(id: string) {
    if (byId.value[id]) {
      return Promise.resolve();
    }
    let run = inflight.get(id);
    if (!run) {
      run = load(id).finally(() => inflight.delete(id));
      inflight.set(id, run);
    }
    return run;
  }

  return { byId, loading, error, get, replace, load, ensureLoaded };
});
