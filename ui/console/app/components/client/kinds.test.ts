import { describe, expect, it } from 'vitest';

import { ClientKind } from '../../../gen/client/v1/client_pb';
import { clientKindNames, clientKinds, kindName } from './kinds';

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
});
