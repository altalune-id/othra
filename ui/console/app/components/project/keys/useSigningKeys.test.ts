import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { SigningKeySchema, SigningKeyStatus } from '../../../../gen/keys/v1/keys_pb';
import { connectError, deferred, setClient, setKnownKeys } from '../../../../test/nuxt-globals';
import { formatWhen, shortKid, useSigningKeys } from './useSigningKeys';

const projectId = 'x6MH9WPWjktMwCBKHy4pF';
const now = new Date('2026-10-18T14:22:00Z');
const at = (iso: string) => timestampFromDate(new Date(iso));

function key(kid: string, status: SigningKeyStatus, created: string, extra: Record<string, unknown> = {}) {
  return create(SigningKeySchema, { publicId: `pk_${kid}`, kid, alg: 'RS256', status, createdAt: at(created), ...extra });
}

const active = key('8b20d4f1aa', SigningKeyStatus.ACTIVE, '2026-10-10T00:00:00Z');
const retiring = key('1c77e0aabb', SigningKeyStatus.RETIRING, '2026-09-30T00:00:00Z', { retireAfter: at('2026-10-24T00:00:00Z') });
const retired = key('9e01b3c5cc', SigningKeyStatus.RETIRED, '2026-09-01T00:00:00Z', { retireAfter: at('2026-10-17T00:00:00Z') });
const next = key('3f9a1c2edd', SigningKeyStatus.NEXT, '2026-10-18T14:22:00Z', { activatesAt: at('2026-10-18T14:32:00Z') });

describe('shortKid and formatWhen', () => {
  it('shortens a kid to 8 characters', () => {
    expect(shortKid('3f9a1c2e9d8b7a6f')).toBe('3f9a1c2e');
    expect(shortKid('abc')).toBe('abc');
  });

  it('shows only the time today and the date with the time on another day', () => {
    const today = new Date(now.getFullYear(), now.getMonth(), now.getDate(), 14, 32);
    const tomorrow = new Date(now.getFullYear(), now.getMonth(), now.getDate() + 1, 9, 5);
    const ref = new Date(now.getFullYear(), now.getMonth(), now.getDate(), 14, 22);
    expect(formatWhen(today, 'en-US', ref)).toBe(new Intl.DateTimeFormat('en-US', { timeStyle: 'short' }).format(today));
    expect(formatWhen(tomorrow, 'en-US', ref)).toBe(new Intl.DateTimeFormat('en-US', { dateStyle: 'medium', timeStyle: 'short' }).format(tomorrow));
  });
});

describe('useSigningKeys', () => {
  let listSigningKeys: ReturnType<typeof vi.fn>;
  let rotateSigningKey: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(now);
    listSigningKeys = vi.fn().mockResolvedValue({ keys: [retired, active, retiring] });
    rotateSigningKey = vi.fn().mockResolvedValue({ key: next });
    setClient('$signingKeyClient', { listSigningKeys, rotateSigningKey });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('loads the keys sorted Next, Active, Retiring, Retired, newest first within a status', async () => {
    const older = key('00000000ee', SigningKeyStatus.RETIRED, '2026-08-01T00:00:00Z');
    listSigningKeys.mockResolvedValue({ keys: [older, retired, active, next, retiring] });
    const k = useSigningKeys(projectId);
    expect(k.loading.value).toBe(true);
    await k.load();
    expect(listSigningKeys).toHaveBeenCalledWith({ projectId });
    expect(k.keys.value.map(x => x.kid)).toEqual(['3f9a1c2edd', '8b20d4f1aa', '1c77e0aabb', '9e01b3c5cc', '00000000ee']);
    expect([k.loading.value, k.loaded.value, k.hasRetired.value, k.noActive.value]).toEqual([false, true, true, false]);
    expect(k.waitingUntil.value).toEqual(new Date('2026-10-18T14:32:00Z'));
    k.dispose();
  });

  it('shows a load error and loads again on retry', async () => {
    setKnownKeys('apperror.GEN900');
    listSigningKeys.mockRejectedValueOnce(connectError('GEN900'));
    const k = useSigningKeys(projectId);
    await k.load();
    expect(k.loadError.value?.message).toBe('apperror.GEN900');
    expect(k.loaded.value).toBe(false);
    await k.load();
    expect(k.loadError.value).toBeNull();
    expect(k.keys.value).toHaveLength(3);
  });

  it('reports no active key, and no waiting key, when the project has none', async () => {
    listSigningKeys.mockResolvedValue({ keys: [retired] });
    const k = useSigningKeys(projectId);
    await k.load();
    expect(k.noActive.value).toBe(true);
    expect(k.waitingUntil.value).toBeNull();
  });

  it('rotates after the confirm, notes the new key and shows it waiting', async () => {
    const k = useSigningKeys(projectId);
    await k.load();
    k.askRotate();
    expect(k.confirming.value).toBe(true);
    listSigningKeys.mockResolvedValue({ keys: [next, active, retiring, retired] });
    const pending = k.confirmRotate();
    expect(k.busy.value).toBe(true);
    await pending;
    expect(rotateSigningKey).toHaveBeenCalledWith({ projectId, immediate: false });
    expect(k.confirming.value).toBe(false);
    expect(k.rotated.value).toEqual({ kid: '3f9a1c2edd', activatesAt: new Date('2026-10-18T14:32:00Z') });
    expect(k.waitingUntil.value).toEqual(new Date('2026-10-18T14:32:00Z'));
    expect(listSigningKeys).toHaveBeenCalledTimes(2);
    k.dispose();
  });

  it('notes a key that is active at once, when it repaired a project with none', async () => {
    const repaired = key('aaaaaaaaff', SigningKeyStatus.ACTIVE, '2026-10-18T14:22:00Z');
    listSigningKeys.mockResolvedValue({ keys: [] });
    rotateSigningKey.mockResolvedValue({ key: repaired });
    const k = useSigningKeys(projectId);
    await k.load();
    k.askRotate();
    await k.confirmRotate();
    expect(k.rotated.value).toEqual({ kid: 'aaaaaaaaff', activatesAt: null });
  });

  it('keeps the confirm open with the server\'s reason when a key is already waiting, and refetches on close', async () => {
    setKnownKeys('apperror.SGK002');
    rotateSigningKey.mockRejectedValueOnce(connectError('SGK002'));
    const k = useSigningKeys(projectId);
    await k.load();
    k.askRotate();
    await k.confirmRotate();
    expect(k.confirming.value).toBe(true);
    expect(k.error.value?.message).toBe('apperror.SGK002');
    expect(k.rotated.value).toBeNull();
    await k.cancelRotate();
    expect(k.confirming.value).toBe(false);
    expect(k.error.value).toBeNull();
    expect(listSigningKeys).toHaveBeenCalledTimes(2);
  });

  it('ignores a second confirm while one is running, and a cancel without a refusal does not refetch', async () => {
    const pending = deferred<{ key: typeof next }>();
    rotateSigningKey.mockReturnValue(pending.promise);
    const k = useSigningKeys(projectId);
    await k.load();
    await k.confirmRotate();
    expect(rotateSigningKey).not.toHaveBeenCalled();
    k.askRotate();
    const first = k.confirmRotate();
    await k.confirmRotate();
    pending.resolve({ key: next });
    await first;
    expect(rotateSigningKey).toHaveBeenCalledTimes(1);
    k.askRotate();
    await k.cancelRotate();
    expect(listSigningKeys).toHaveBeenCalledTimes(2);
    k.dispose();
  });

  it('refetches 70 s after a waiting key takes over, then every 30 s at most 5 times', async () => {
    listSigningKeys.mockResolvedValue({ keys: [next, active] });
    const k = useSigningKeys(projectId);
    await k.load();
    await vi.advanceTimersByTimeAsync(10 * 60_000 + 69_000);
    expect(listSigningKeys).toHaveBeenCalledTimes(1);
    await vi.advanceTimersByTimeAsync(1_000);
    expect(listSigningKeys).toHaveBeenCalledTimes(2);
    await vi.advanceTimersByTimeAsync(5 * 30_000);
    expect(listSigningKeys).toHaveBeenCalledTimes(7);
    await vi.advanceTimersByTimeAsync(10 * 30_000);
    expect(listSigningKeys).toHaveBeenCalledTimes(7);
    k.dispose();
  });

  it('stops refetching once the key is active, and on dispose', async () => {
    const promoted = key('3f9a1c2edd', SigningKeyStatus.ACTIVE, '2026-10-18T14:22:00Z');
    listSigningKeys.mockResolvedValueOnce({ keys: [next, active] }).mockResolvedValue({ keys: [promoted] });
    const k = useSigningKeys(projectId);
    await k.load();
    await vi.advanceTimersByTimeAsync(11 * 60_000 + 10_000);
    expect(listSigningKeys).toHaveBeenCalledTimes(2);
    expect(k.waitingUntil.value).toBeNull();
    await vi.advanceTimersByTimeAsync(5 * 60_000);
    expect(listSigningKeys).toHaveBeenCalledTimes(2);

    listSigningKeys.mockResolvedValue({ keys: [next, active] });
    const other = useSigningKeys(projectId);
    await other.load();
    other.dispose();
    await vi.advanceTimersByTimeAsync(20 * 60_000);
    expect(listSigningKeys).toHaveBeenCalledTimes(3);
  });

  it('refetches soon when the waiting time has already passed', async () => {
    const late = key('3f9a1c2edd', SigningKeyStatus.NEXT, '2026-10-18T14:00:00Z', { activatesAt: at('2026-10-18T14:10:00Z') });
    listSigningKeys.mockResolvedValue({ keys: [late, active] });
    const k = useSigningKeys(projectId);
    await k.load();
    await vi.advanceTimersByTimeAsync(29_000);
    expect(listSigningKeys).toHaveBeenCalledTimes(1);
    await vi.advanceTimersByTimeAsync(1_000);
    expect(listSigningKeys).toHaveBeenCalledTimes(2);
    k.dispose();
  });
});
