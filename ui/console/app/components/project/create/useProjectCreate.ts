import type { ShownError } from '#base/lib/apperror/message';

import { create } from '@bufbuild/protobuf';
import { watch } from 'vue';
import { useValidation } from '#base/composables/useValidation';
import { CreateProjectRequestSchema } from '#gen/project/v1/project_pb';
import { slugify, useIssuerHandle } from '~/components/project/issuer/useIssuerHandle';
import { useOrgsStore } from '~/stores/orgs';
import { useProjectsStore } from '~/stores/projects';
import { useUIStore } from '~/stores/ui';

export function useProjectCreate(orgId: string) {
  const name = ref('');
  const slug = ref('');
  const busy = ref(false);
  const error = ref<ShownError | null>(null);

  const projects = useProjectsStore();
  const orgs = useOrgsStore();
  const ui = useUIStore();
  const handle = useIssuerHandle({ orgId, initial: '' });
  const validation = useValidation(CreateProjectRequestSchema, {
    fields: ['name', 'slug', 'issuer_handle'],
    codes: { PRJ002: 'slug', PRJ003: 'slug', PRJ004: 'name', PRJ005: 'issuer_handle', PRJ006: 'issuer_handle' },
  });
  const blocked = new Set(['checking', 'taken', 'reserved', 'invalid', 'locked']);
  const handleReady = computed(() => !!handle.value.value.trim() && !blocked.has(handle.status.value));
  const canSubmit = computed(() => !busy.value && !!name.value.trim() && handleReady.value);

  function prefill() {
    handle.follow(orgs.items.find(o => o.id === orgId)?.slug ?? '', slug.value.trim() || slugify(name.value));
  }
  watch([name, slug], prefill, { flush: 'sync' });

  function setSlug(value: string) {
    slug.value = value;
  }
  const { $projectClient: client } = useNuxtApp();

  const open = computed(() => ui.projectCreateOpen);

  function hide() {
    if (busy.value) {
      return;
    }
    ui.closeProjectCreate();
    error.value = null;
    validation.clear();
  }

  async function submit() {
    if (busy.value) {
      return;
    }
    const req = { orgId, name: name.value.trim(), slug: slug.value.trim(), issuerHandle: handle.value.value.trim() };
    if (!validation.check(create(CreateProjectRequestSchema, req))) {
      error.value = validation.formError.value;
      return;
    }
    if (!handleReady.value) {
      return;
    }
    busy.value = true;
    error.value = null;
    try {
      const res = await client.createProject(req);
      const project = res.project;
      if (project) {
        projects.add({ id: project.id, orgId: project.orgId, slug: project.slug, name: project.name });
        ui.closeProjectCreate();
        name.value = '';
        slug.value = '';
        handle.reset('');
      }
    } catch (e) {
      error.value = validation.fromServer(e, 'error.create_project');
    } finally {
      busy.value = false;
    }
  }

  return { open, name, slug, handle, busy, canSubmit, error, fieldErrors: validation.errors, setSlug, show: ui.openProjectCreate, hide, submit };
}
