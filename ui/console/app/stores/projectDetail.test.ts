import { create } from '@bufbuild/protobuf';
import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { ProjectSchema } from '#gen/project/v1/project_pb';
import { connectError, deferred, setClient, setKnownKeys } from '#test/nuxt-globals';
import { useProjectDetailStore } from './projectDetail';

const id = 'x6MH9WPWjktMwCBKHy4pF';
const project = create(ProjectSchema, { id, name: 'Web', issuerHandle: 'acme-web', issuerUrl: 'https://auth.test/p/acme-web' });

describe('useProjectDetailStore', () => {
  let getProject: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    setActivePinia(createPinia());
    getProject = vi.fn().mockResolvedValue({ project });
    setClient('$projectClient', { getProject });
  });

  it('loads one project by id and keeps it', async () => {
    const detail = useProjectDetailStore();
    expect(detail.get(id)).toBeUndefined();
    await detail.load(id);
    expect(getProject).toHaveBeenCalledWith({ id });
    expect(detail.get(id)?.issuerHandle).toBe('acme-web');
    expect(detail.error).toBeNull();
  });

  it('shares an in-flight load and skips a project already loaded', async () => {
    const pending = deferred<{ project: typeof project }>();
    getProject.mockReturnValueOnce(pending.promise);
    const detail = useProjectDetailStore();
    const a = detail.ensureLoaded(id);
    const b = detail.ensureLoaded(id);
    pending.resolve({ project });
    await Promise.all([a, b]);
    await detail.ensureLoaded(id);
    expect(getProject).toHaveBeenCalledTimes(1);
  });

  it('replaces a project with the one an update returned', async () => {
    const detail = useProjectDetailStore();
    await detail.load(id);
    detail.replace(create(ProjectSchema, { ...project, displayName: 'Acme' }));
    expect(detail.get(id)?.displayName).toBe('Acme');
  });

  it('keeps nothing when the response has no project, and shows load errors', async () => {
    setKnownKeys('apperror.PRJ001');
    getProject.mockResolvedValueOnce({}).mockRejectedValueOnce(connectError('PRJ001')).mockRejectedValueOnce(new Error('boom'));
    const detail = useProjectDetailStore();
    await detail.load(id);
    expect(detail.get(id)).toBeUndefined();
    await detail.load(id);
    expect(detail.error).toEqual({ message: 'apperror.PRJ001', reference: 'error.ref|{"id":"req-1"}' });
    await detail.load(id);
    expect(detail.error).toEqual({ message: 'error.load_project' });
    expect(detail.loading).toBe(false);
  });
});
