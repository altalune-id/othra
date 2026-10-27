import type { ShownError } from '#base/lib/apperror/message';
import type { Link, ResourceServer } from '#gen/resourceserver/v1/resourceserver_pb';
import { create } from '@bufbuild/protobuf';
import { useErrorMessage } from '#base/composables/useErrorMessage';
import { useValidation } from '#base/composables/useValidation';
import { ResourceServerState, SetResourceServerAdmissionRequestSchema } from '#gen/resourceserver/v1/resourceserver_pb';

// admissible reports whether a resource server may accept apps without registration: verified, or trusted without verification.
export function admissible(rs: Pick<ResourceServer, 'state' | 'trusted'>): boolean {
  return rs.trusted || rs.state === ResourceServerState.VERIFIED;
}

// parseHosts reads one host per line: trimmed, lower-cased, without blanks or repeats, in the order typed.
export function parseHosts(text: string): string[] {
  const hosts: string[] = [];
  for (const line of text.split('\n')) {
    const host = line.trim().toLowerCase();
    if (host && !hosts.includes(host)) {
      hosts.push(host);
    }
  }
  return hosts;
}

// isCimdLink reports a link to an app that signed in with a client metadata document instead of a registration.
export function isCimdLink(link: Pick<Link, 'clientMetadataUrl'>): boolean {
  return link.clientMetadataUrl !== '';
}

// useCimd edits a resource server's admission of apps without registration and revokes the apps that signed in that way.
export function useCimd(projectId: string, resourceServerId: string, onServer: (rs: ResourceServer) => void, onRevoked: (clientId: string) => void) {
  const { $resourceServerClient: rsClient, $clientClient: clientClient } = useNuxtApp();
  const errorMessage = useErrorMessage();
  const { t } = useI18n();
  const validation = useValidation(SetResourceServerAdmissionRequestSchema, { fields: ['hosts', 'scopes'], codes: { RSV010: 'hosts' } });

  const accept = ref(false);
  const hosts = ref('');
  const allScopes = ref(true);
  const picked = ref<string[]>([]);
  const catalog = ref<string[]>([]);
  const busy = ref(false);
  const saved = ref(false);
  const error = ref<ShownError | null>(null);
  const hostsError = computed(() => validation.errors.value.hosts ?? '');
  const revoking = ref<Link | null>(null);
  const revokeBusy = ref(false);
  const revokeError = ref<ShownError | null>(null);

  function reset(rs: ResourceServer) {
    accept.value = rs.cimd?.accept ?? false;
    hosts.value = (rs.cimd?.hosts ?? []).join('\n');
    allScopes.value = rs.cimd?.allScopes ?? true;
    picked.value = [...(rs.cimd?.scopes ?? [])];
    catalog.value = [...rs.scopes];
  }

  async function save() {
    if (busy.value) {
      return;
    }
    saved.value = false;
    error.value = null;
    validation.clear();
    const scopes = catalog.value.filter(s => picked.value.includes(s));
    if (accept.value && !allScopes.value && scopes.length === 0) {
      error.value = { message: t('apperror.RSV009_empty') };
      return;
    }
    const req = {
      projectId,
      resourceServerId,
      accept: accept.value,
      hosts: parseHosts(hosts.value),
      allScopes: allScopes.value,
      scopes: allScopes.value ? [] : scopes,
    };
    if (!validation.check(create(SetResourceServerAdmissionRequestSchema, req))) {
      error.value = validation.formError.value;
      return;
    }
    busy.value = true;
    try {
      const res = await rsClient.setResourceServerAdmission(req);
      if (res.resourceServer) {
        onServer(res.resourceServer);
        reset(res.resourceServer);
      }
      saved.value = true;
    } catch (e) {
      error.value = validation.fromServer(e, 'error.save_rs');
    } finally {
      busy.value = false;
    }
  }

  function askRevoke(link: Link) {
    revokeError.value = null;
    revoking.value = link;
  }

  function cancelRevoke() {
    if (!revokeBusy.value) {
      revoking.value = null;
      revokeError.value = null;
    }
  }

  async function confirmRevoke() {
    const link = revoking.value;
    if (!link || revokeBusy.value) {
      return;
    }
    revokeBusy.value = true;
    revokeError.value = null;
    try {
      await clientClient.deleteClient({ projectId, clientId: link.clientId });
      onRevoked(link.clientId);
      revoking.value = null;
    } catch (e) {
      revokeError.value = errorMessage(e, 'error.delete_client');
    } finally {
      revokeBusy.value = false;
    }
  }

  return { accept, hosts, allScopes, picked, catalog, busy, saved, error, hostsError, revoking, revokeBusy, revokeError, reset, save, askRevoke, cancelRevoke, confirmRevoke };
}
