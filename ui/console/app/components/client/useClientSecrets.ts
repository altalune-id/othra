import type { Client, ClientSecret } from '~~/gen/client/v1/client_pb';
import type { RevealedSecret } from './useClientForm';
import type { ShownError } from '~/lib/apperror/message';
import { timestampDate } from '@bufbuild/protobuf/wkt';
import { SecretGrace } from '~~/gen/client/v1/client_pb';
import { useErrorMessage } from '~/composables/useErrorMessage';

export interface SecretRow {
  id: string;
  hint: string;
  old: boolean;
  created: Date | null;
  expires: Date | null;
}

export interface SecretRef {
  id: string;
  hint: string;
}

export const graceOptions = [
  { value: SecretGrace.NOW, labelKey: 'secrets.grace_0' },
  { value: SecretGrace.HOUR, labelKey: 'secrets.grace_1h' },
  { value: SecretGrace.DAY, labelKey: 'secrets.grace_1d' },
  { value: SecretGrace.WEEK, labelKey: 'secrets.grace_7d' },
];

// secretRows lists a client's live secrets, the current one first, then the old one that still works until it expires.
export function secretRows(secrets: ClientSecret[]): SecretRow[] {
  return secrets
    .map(s => ({
      id: s.id,
      hint: s.hint,
      old: !!s.expiresAt,
      created: s.createdAt ? timestampDate(s.createdAt) : null,
      expires: s.expiresAt ? timestampDate(s.expiresAt) : null,
    }))
    .sort((a, b) => Number(a.old) - Number(b.old) || (b.created?.getTime() ?? 0) - (a.created?.getTime() ?? 0));
}

// useClientSecrets generates, rotates and revokes a web client's secrets; a new secret is held only until the dialog is done.
export function useClientSecrets(projectId: string, clientId: string, onClient: (client: Client) => void) {
  const { $clientClient: client } = useNuxtApp();
  const errorMessage = useErrorMessage();

  const rotating = ref(false);
  const grace = ref<SecretGrace>(SecretGrace.DAY);
  const revoking = ref<SecretRef | null>(null);
  const busy = ref(false);
  const error = ref<ShownError | null>(null);
  const revealed = ref<RevealedSecret | null>(null);

  function update(next: Client | undefined) {
    if (next) {
      onClient(next);
    }
  }

  async function generate() {
    if (busy.value) {
      return;
    }
    busy.value = true;
    error.value = null;
    try {
      const res = await client.generateClientSecret({ projectId, clientId });
      update(res.client);
      revealed.value = { clientId, secret: res.secret };
    } catch (e) {
      error.value = errorMessage(e, 'error.generate_secret');
    } finally {
      busy.value = false;
    }
  }

  function askRotate() {
    error.value = null;
    grace.value = SecretGrace.DAY;
    rotating.value = true;
  }

  function cancelRotate() {
    if (!busy.value) {
      rotating.value = false;
      error.value = null;
    }
  }

  async function confirmRotate() {
    if (!rotating.value || busy.value) {
      return;
    }
    busy.value = true;
    error.value = null;
    try {
      const res = await client.rotateClientSecret({ projectId, clientId, grace: grace.value });
      update(res.client);
      rotating.value = false;
      revealed.value = { clientId, secret: res.secret };
    } catch (e) {
      error.value = errorMessage(e, 'error.rotate_secret');
    } finally {
      busy.value = false;
    }
  }

  function askRevoke(row: SecretRef) {
    error.value = null;
    revoking.value = { id: row.id, hint: row.hint };
  }

  function cancelRevoke() {
    if (!busy.value) {
      revoking.value = null;
      error.value = null;
    }
  }

  async function confirmRevoke() {
    const row = revoking.value;
    if (!row || busy.value) {
      return;
    }
    busy.value = true;
    error.value = null;
    try {
      const res = await client.revokeClientSecret({ projectId, clientId, secretId: row.id });
      update(res.client);
      revoking.value = null;
    } catch (e) {
      error.value = errorMessage(e, 'error.revoke_secret');
    } finally {
      busy.value = false;
    }
  }

  function forget() {
    revealed.value = null;
  }

  return { rotating, grace, revoking, busy, error, revealed, generate, askRotate, cancelRotate, confirmRotate, askRevoke, cancelRevoke, confirmRevoke, forget };
}
