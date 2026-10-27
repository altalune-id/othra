import type { ShownError } from '#base/lib/apperror/message';
import type { ClientSecret } from '#gen/client/v1/client_pb';
import { timestampDate } from '@bufbuild/protobuf/wkt';
import { useErrorMessage } from '#base/composables/useErrorMessage';
import { SecretGrace } from '#gen/client/v1/client_pb';

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

export interface OwnerSecret {
  ownerId: string;
  secret: string;
}

export interface SecretAnswer<T> {
  owner?: T;
  secret?: string;
}

export interface SecretCalls<T> {
  generate: () => Promise<SecretAnswer<T>>;
  rotate: (grace: SecretGrace) => Promise<SecretAnswer<T>>;
  revoke: (secretId: string) => Promise<SecretAnswer<T>>;
}

// useSecrets generates, rotates and revokes an owner's secrets through calls; a new secret is held only until the dialog is done.
export function useSecrets<T>(ownerId: string, calls: SecretCalls<T>, onOwner: (owner: T) => void) {
  const errorMessage = useErrorMessage();

  const rotating = ref(false);
  const grace = ref<SecretGrace>(SecretGrace.DAY);
  const revoking = ref<SecretRef | null>(null);
  const busy = ref(false);
  const error = ref<ShownError | null>(null);
  const revealed = ref<OwnerSecret | null>(null);

  function update(next: T | undefined) {
    if (next) {
      onOwner(next);
    }
  }

  async function generate() {
    if (busy.value) {
      return;
    }
    busy.value = true;
    error.value = null;
    try {
      const res = await calls.generate();
      update(res.owner);
      revealed.value = { ownerId, secret: res.secret ?? '' };
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
      const res = await calls.rotate(grace.value);
      update(res.owner);
      rotating.value = false;
      revealed.value = { ownerId, secret: res.secret ?? '' };
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
      update((await calls.revoke(row.id)).owner);
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
