import { describe, expect, it } from 'vitest';

import { ClientKind, ClientRegistration } from '#gen/client/v1/client_pb';
import { clientKindNames, clientKinds, isCimdClient, isSystemClient, kindName } from './kinds';

describe('clientKinds', () => {
  it('offers a web app with a secret and a CLI app without one', () => {
    expect(clientKindNames).toEqual(['web', 'cli']);
    expect([clientKinds.web.value, clientKinds.web.secret, clientKinds.web.postLogout]).toEqual([ClientKind.WEB, true, true]);
    expect([clientKinds.cli.value, clientKinds.cli.secret, clientKinds.cli.postLogout]).toEqual([ClientKind.CLI, false, false]);
  });

  it('maps the proto kind onto a console kind, reading anything unknown as a web app', () => {
    expect(kindName(ClientKind.CLI)).toBe('cli');
    expect(kindName(ClientKind.WEB)).toBe('web');
    expect(kindName(ClientKind.UNSPECIFIED)).toBe('web');
  });

  it('tells a CIMD client from a registered one', () => {
    expect(isCimdClient({ registration: ClientRegistration.CIMD })).toBe(true);
    expect(isCimdClient({ registration: ClientRegistration.REGISTERED })).toBe(false);
    expect(isCimdClient({ registration: ClientRegistration.UNSPECIFIED })).toBe(false);
    expect(isSystemClient({ registration: ClientRegistration.SYSTEM })).toBe(true);
    expect(isSystemClient({ registration: ClientRegistration.REGISTERED })).toBe(false);
  });
});
