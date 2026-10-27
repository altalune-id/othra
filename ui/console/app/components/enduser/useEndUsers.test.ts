import { create } from '@bufbuild/protobuf';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { EndUserSchema } from '#gen/enduser/v1/enduser_pb';
import { connectError, deferred, setClient, setKnownKeys } from '#test/nuxt-globals';
import { useEndUserList } from './useEndUsers';

const projectId = 'x6MH9WPWjktMwCBKHy4pF';
const ana = create(EndUserSchema, { id: 'F5pLm2XwQ8kRt3VnZ7bYc', email: 'ana@acme.test' });
const budi = create(EndUserSchema, { id: 'W1zRq6NvK3pLx8TmB4cHd', email: 'budi@acme.test' });

describe('useEndUserList', () => {
  let listEndUsers: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    listEndUsers = vi.fn().mockResolvedValueOnce({ endUsers: [ana], next: 'dPm7fmQRJKPVw8PxrgdrN' }).mockResolvedValueOnce({ endUsers: [budi], next: '' });
    setClient('$endUserClient', { listEndUsers });
  });

  it('loads a page, then the next one, filtered by the trimmed email', async () => {
    const l = useEndUserList(projectId);
    l.query.value = ' ana ';
    await l.load();
    expect(listEndUsers).toHaveBeenLastCalledWith({ projectId, query: 'ana', limit: 50, after: '' });
    expect(l.next.value).toBe('dPm7fmQRJKPVw8PxrgdrN');
    await l.more();
    expect(listEndUsers).toHaveBeenLastCalledWith({ projectId, query: 'ana', limit: 50, after: 'dPm7fmQRJKPVw8PxrgdrN' });
    expect(l.users.value.map(u => u.email)).toEqual(['ana@acme.test', 'budi@acme.test']);
    expect(await l.more()).toBe(false);
  });

  it('keeps paging the shown search even after the search box changes', async () => {
    const l = useEndUserList(projectId);
    l.query.value = 'ana';
    await l.load();
    l.query.value = 'budi';
    await l.more();
    expect(listEndUsers).toHaveBeenLastCalledWith({ projectId, query: 'ana', limit: 50, after: 'dPm7fmQRJKPVw8PxrgdrN' });
    expect(l.shown.value, 'the empty message follows the shown search, not the box (N8)').toBe('ana');
  });

  it('shows a load error', async () => {
    setKnownKeys('apperror.GEN900');
    listEndUsers.mockReset().mockRejectedValueOnce(connectError('GEN900'));
    const l = useEndUserList(projectId);
    await l.load();
    expect(l.loadError.value?.message).toBe('apperror.GEN900');
    expect(l.loading.value).toBe(false);
  });

  it('drops an older answer that lands after a newer search', async () => {
    const slow = deferred<object>();
    listEndUsers.mockReset().mockReturnValueOnce(slow.promise).mockResolvedValueOnce({ endUsers: [budi], next: '' });
    const l = useEndUserList(projectId);
    const first = l.load();
    l.query.value = 'budi';
    await l.load();
    slow.resolve({ endUsers: [ana], next: 'x' });
    expect(await first).toBe(false);
    expect(l.users.value.map(u => u.email)).toEqual(['budi@acme.test']);
    const late = deferred<object>();
    listEndUsers.mockReturnValueOnce(late.promise).mockResolvedValueOnce({ endUsers: [], next: '' });
    const stale = l.load();
    await l.load();
    late.reject(connectError('GEN900'));
    expect(await stale).toBe(false);
    expect(l.loadError.value).toBeNull();
  });
});
