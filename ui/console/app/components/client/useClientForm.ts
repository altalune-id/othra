import type { Client } from '~~/gen/client/v1/client_pb';
import type { ClientFields } from './clientForm';
import type { ClientKindName } from './kinds';
import type { UriList, UriProblem } from './redirectUris';
import type { ShownError } from '~/lib/apperror/message';
import { create } from '@bufbuild/protobuf';
import { CreateClientRequestSchema, UpdateClientRequestSchema } from '~~/gen/client/v1/client_pb';
import { useErrorMessage } from '~/composables/useErrorMessage';
import { useValidation } from '~/composables/useValidation';
import { errorCode, errorMeta } from '~/lib/apperror/message';
import { consentValue, emptyFields, fieldsOf } from './clientForm';
import { clientKinds } from './kinds';
import { checkUriList, serverRow, usesLocalhost } from './redirectUris';

export type UriListName = 'redirect' | 'postLogout';

export interface RevealedSecret {
  clientId: string;
  secret: string;
}

interface Lists {
  redirect: UriList;
  postLogout: UriList;
}

// useClientForm holds the create and edit form of an OAuth client; a new web app gets its first secret right after create.
export function useClientForm(projectId: string, saved?: Client) {
  const { $clientClient: client } = useNuxtApp();
  const { t } = useI18n();
  const errorMessage = useErrorMessage();
  const validation = useValidation(saved ? UpdateClientRequestSchema : CreateClientRequestSchema, { fields: ['name', 'redirect_uris', 'post_logout_redirect_uris'] });

  const fields = ref<ClientFields>(saved ? fieldsOf(saved) : emptyFields());
  const uriErrors = ref<Record<UriListName, string[]>>({ redirect: [], postLogout: [] });
  const busy = ref(false);
  const error = ref<ShownError | null>(null);
  const done = ref(false);
  const secretError = ref<ShownError | null>(null);
  const revealed = ref<RevealedSecret | null>(null);
  const result = ref<Client | null>(null);

  const kind = computed(() => clientKinds[fields.value.kind]);
  const warnings = computed<Record<UriListName, boolean[]>>(() => ({
    redirect: fields.value.redirectUris.map(usesLocalhost),
    postLogout: fields.value.postLogoutUris.map(usesLocalhost),
  }));
  const nameError = computed(() => validation.errors.value.name ?? '');

  function describe(problem: UriProblem | null): string {
    if (!problem) {
      return '';
    }
    return t(`apperror.CLT002_${problem.reason}`, problem.canonical ? { canonical: problem.canonical } : {});
  }

  function lists(): Lists {
    const name = fields.value.kind;
    return {
      redirect: checkUriList(name, fields.value.redirectUris, true),
      postLogout: checkUriList(name, kind.value.postLogout ? fields.value.postLogoutUris : [], false),
    };
  }

  function setKind(next: ClientKindName) {
    if (saved) {
      return;
    }
    fields.value = { ...fields.value, kind: next, postLogoutUris: clientKinds[next].postLogout ? fields.value.postLogoutUris : [] };
    uriErrors.value = { redirect: [], postLogout: [] };
  }

  function clearRow(list: UriListName, index: number) {
    if (!uriErrors.value[list][index]) {
      return;
    }
    const row = [...uriErrors.value[list]];
    row[index] = '';
    uriErrors.value = { ...uriErrors.value, [list]: row };
  }

  function removeRow(list: UriListName, index: number) {
    const key = list === 'redirect' ? 'redirectUris' : 'postLogoutUris';
    const rows = fields.value[key].filter((_, i) => i !== index);
    fields.value = { ...fields.value, [key]: list === 'redirect' && rows.length === 0 ? [''] : rows };
    uriErrors.value = { ...uriErrors.value, [list]: uriErrors.value[list].filter((_, i) => i !== index) };
  }

  function request(l: Lists) {
    const f = fields.value;
    return {
      projectId,
      name: f.name.trim(),
      redirectUris: l.redirect.uris,
      postLogoutRedirectUris: l.postLogout.uris,
      refreshTokens: f.refreshTokens,
      consentPolicy: consentValue(f.consent),
    };
  }

  function check(): Lists | null {
    error.value = null;
    done.value = false;
    const l = lists();
    uriErrors.value = { redirect: l.redirect.problems.map(describe), postLogout: l.postLogout.problems.map(describe) };
    const req = request(l);
    const valid = saved
      ? validation.check(create(UpdateClientRequestSchema, { ...req, clientId: saved.id }))
      : validation.check(create(CreateClientRequestSchema, { ...req, kind: kind.value.value }));
    const uriOk = [...l.redirect.problems, ...l.postLogout.problems].every(p => p === null);
    if (!valid) {
      error.value = validation.formError.value;
    }
    return valid && uriOk ? l : null;
  }

  function refused(e: unknown, l: Lists, fallback: string): ShownError | null {
    if (errorCode(e) === 'CLT002') {
      const meta = errorMeta(e);
      const list: UriListName = (meta.field ?? '').startsWith('post_logout') ? 'postLogout' : 'redirect';
      const row = serverRow(meta.field ?? '', meta.uri ?? '', l[list]);
      if (row >= 0) {
        const rows = [...uriErrors.value[list]];
        rows[row] = errorMessage(e, fallback).message;
        uriErrors.value = { ...uriErrors.value, [list]: rows };
        return null;
      }
    }
    return validation.fromServer(e, fallback);
  }

  async function submit(): Promise<Client | null> {
    if (busy.value) {
      return null;
    }
    const l = check();
    if (!l) {
      return null;
    }
    busy.value = true;
    try {
      const res = saved
        ? await client.updateClient({ ...request(l), clientId: saved.id })
        : await client.createClient({ ...request(l), kind: kind.value.value });
      result.value = res.client ?? null;
      done.value = true;
      if (!saved && res.client && kind.value.secret) {
        await generate(res.client.id);
      }
      return result.value;
    } catch (e) {
      error.value = refused(e, l, 'error.save_client');
      return null;
    } finally {
      busy.value = false;
    }
  }

  async function generate(clientId: string) {
    secretError.value = null;
    try {
      const res = await client.generateClientSecret({ projectId, clientId });
      result.value = res.client ?? result.value;
      revealed.value = { clientId, secret: res.secret };
    } catch (e) {
      secretError.value = errorMessage(e, 'error.generate_secret');
    }
  }

  function forget() {
    revealed.value = null;
  }

  return { fields, kind, warnings, uriErrors, nameError, busy, error, done, secretError, revealed, result, setKind, clearRow, removeRow, submit, forget };
}
