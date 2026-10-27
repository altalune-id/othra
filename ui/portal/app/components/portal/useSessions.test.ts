import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { SessionSchema, SignInMethodKind } from '#gen/account/v1/account_pb';
import { connectError, setClient, setKnownKeys } from '#test/nuxt-globals';
import { useSessions } from './useSessions';

const mac = 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36';
const other = create(SessionSchema, { id: 'H3nVb8XqT1wLp6RkZ2mJc', method: SignInMethodKind.GOOGLE, userAgent: 'curl/8', authTime: timestampFromDate(new Date('2026-10-20T00:00:00Z')) });
const here = create(SessionSchema, { id: 'Q7mZp2LkV9xRt4NbW8cYd', method: SignInMethodKind.EMAIL_LINK, userAgent: mac, current: true });

describe('useSessions', () => {
  let client: Record<string, ReturnType<typeof vi.fn>>;

  beforeEach(() => {
    client = {
      listSessions: vi.fn().mockResolvedValue({ sessions: [other, here] }),
      endSession: vi.fn().mockResolvedValue({}),
      endOtherSessions: vi.fn().mockResolvedValue({ ended: 1 }),
    };
    setClient('$accountClient', client);
  });

  it('lists the current session first', async () => {
    const s = useSessions();
    await s.load();
    expect(s.sessions.value.map(x => x.id)).toEqual(['Q7mZp2LkV9xRt4NbW8cYd', 'H3nVb8XqT1wLp6RkZ2mJc']);
    expect(s.others.value.map(x => x.id)).toEqual(['H3nVb8XqT1wLp6RkZ2mJc']);
  });

  it('shows a load error', async () => {
    setKnownKeys('apperror.GEN900');
    client.listSessions!.mockRejectedValueOnce(connectError('GEN900'));
    const s = useSessions();
    await s.load();
    expect(s.loadError.value?.message).toBe('apperror.GEN900');
  });

  it('ends one session, then every other one', async () => {
    const s = useSessions();
    await s.load();
    s.end.ask(s.sessions.value[1]!);
    await s.end.confirm();
    expect(client.endSession).toHaveBeenCalledWith({ sessionId: 'H3nVb8XqT1wLp6RkZ2mJc' });
    expect(s.sessions.value.map(x => x.id)).toEqual(['Q7mZp2LkV9xRt4NbW8cYd']);
    await s.load();
    s.endOthers.ask(true);
    await s.endOthers.confirm();
    expect(client.endOtherSessions).toHaveBeenCalled();
    expect(s.others.value).toEqual([]);
  });
});
