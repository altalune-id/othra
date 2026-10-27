import type { ShownError } from '#base/lib/apperror/message';
import type { Link } from '#gen/resourceserver/v1/resourceserver_pb';
import { useErrorMessage } from '#base/composables/useErrorMessage';
import { ClientRegistration } from '#gen/client/v1/client_pb';
import { ResourceServerState } from '#gen/resourceserver/v1/resourceserver_pb';

export type LinkSide = 'client' | 'resourceServer';

// LinkTarget is one thing the dialog can link to: a resource server on the client page, a client on the resource server page.
export interface LinkTarget {
  id: string;
  name: string;
  detail: string;
  scopes: string[];
  unverified: boolean;
}

export interface LinkOwner {
  side: LinkSide;
  id: string;
  scopes?: string[];
}

// linkUnverified reports a link to a resource server that is neither verified nor trusted.
export function linkUnverified(state: ResourceServerState, trusted: boolean): boolean {
  return !trusted && state !== ResourceServerState.VERIFIED;
}

// useLinks lists one client's or one resource server's links and links, edits and unlinks them.
export function useLinks(projectId: string, owner: LinkOwner) {
  const { $resourceServerClient: rsClient, $clientClient: clientClient } = useNuxtApp();
  const errorMessage = useErrorMessage();

  const links = ref<Link[]>([]);
  const targets = ref<LinkTarget[]>([]);
  const loading = ref(true);
  const loadError = ref<ShownError | null>(null);
  const editing = ref<'' | 'new' | 'edit'>('');
  const targetId = ref('');
  const picked = ref<string[]>([]);
  const pickError = ref(false);
  const unlinking = ref<Link | null>(null);
  const busy = ref(false);
  const error = ref<ShownError | null>(null);

  const linkedIds = computed(() => new Set(links.value.map(l => (owner.side === 'client' ? l.resourceServerId : l.clientId))));
  const available = computed(() => targets.value.filter(x => !linkedIds.value.has(x.id)));
  const target = computed(() => targets.value.find(x => x.id === targetId.value) ?? null);
  const ownerScopes = ref<string[]>(owner.scopes ? [...owner.scopes] : []);
  const scopeOptions = computed(() => (owner.side === 'client' ? target.value?.scopes ?? [] : ownerScopes.value));

  async function loadTargets() {
    if (owner.side === 'client') {
      const res = await rsClient.listResourceServers({ projectId });
      targets.value = res.resourceServers.map(rs => ({ id: rs.id, name: rs.name, detail: rs.identifier, scopes: [...rs.scopes], unverified: linkUnverified(rs.state, rs.trusted) }));
    } else {
      const res = await clientClient.listClients({ projectId });
      targets.value = res.clients.filter(c => c.registration !== ClientRegistration.CIMD).map(c => ({ id: c.id, name: c.name, detail: c.id, scopes: [], unverified: false }));
    }
  }

  async function load() {
    loadError.value = null;
    try {
      const which = owner.side === 'client' ? { case: 'clientId' as const, value: owner.id } : { case: 'resourceServerId' as const, value: owner.id };
      const [res] = await Promise.all([rsClient.listLinks({ projectId, target: which }), loadTargets()]);
      links.value = [...res.links];
    } catch (e) {
      loadError.value = errorMessage(e, 'error.load_links');
    } finally {
      loading.value = false;
    }
  }

  function drop(id: string) {
    links.value = links.value.filter(l => otherId(l) !== id);
  }

  function setOwnerScopes(scopes: string[]) {
    ownerScopes.value = [...scopes];
  }

  function otherId(link: Link): string {
    return owner.side === 'client' ? link.resourceServerId : link.clientId;
  }

  function askLink() {
    error.value = null;
    pickError.value = false;
    targetId.value = available.value[0]?.id ?? '';
    picked.value = [];
    editing.value = 'new';
  }

  function pickTarget(id: string) {
    targetId.value = id;
    picked.value = [];
    pickError.value = false;
  }

  function askEdit(link: Link) {
    error.value = null;
    pickError.value = false;
    targetId.value = otherId(link);
    picked.value = [...link.scopes];
    editing.value = 'edit';
  }

  function cancelEdit() {
    if (!busy.value) {
      editing.value = '';
      error.value = null;
    }
  }

  function ids() {
    return owner.side === 'client'
      ? { clientId: owner.id, resourceServerId: targetId.value }
      : { clientId: targetId.value, resourceServerId: owner.id };
  }

  async function confirmEdit() {
    if (!editing.value || busy.value || !targetId.value) {
      return;
    }
    const scopes = scopeOptions.value.filter(s => picked.value.includes(s));
    pickError.value = scopes.length === 0;
    if (pickError.value) {
      return;
    }
    busy.value = true;
    error.value = null;
    try {
      const req = { projectId, ...ids(), scopes };
      const res = editing.value === 'new' ? await rsClient.linkClient(req) : await rsClient.updateLink(req);
      const saved = res.link;
      if (saved) {
        const rest = links.value.filter(l => otherId(l) !== otherId(saved));
        links.value = [...rest, saved];
      }
      editing.value = '';
    } catch (e) {
      error.value = errorMessage(e, 'error.save_link');
    } finally {
      busy.value = false;
    }
  }

  function askUnlink(link: Link) {
    error.value = null;
    unlinking.value = link;
  }

  function cancelUnlink() {
    if (!busy.value) {
      unlinking.value = null;
      error.value = null;
    }
  }

  async function confirmUnlink() {
    const link = unlinking.value;
    if (!link || busy.value) {
      return;
    }
    busy.value = true;
    error.value = null;
    try {
      await rsClient.unlinkClient({ projectId, clientId: link.clientId, resourceServerId: link.resourceServerId });
      links.value = links.value.filter(l => otherId(l) !== otherId(link));
      unlinking.value = null;
    } catch (e) {
      error.value = errorMessage(e, 'error.save_link');
    } finally {
      busy.value = false;
    }
  }

  return {
    links,
    available,
    target,
    scopeOptions,
    loading,
    loadError,
    editing,
    targetId,
    picked,
    pickError,
    unlinking,
    busy,
    error,
    load,
    drop,
    setOwnerScopes,
    askLink,
    pickTarget,
    askEdit,
    cancelEdit,
    confirmEdit,
    askUnlink,
    cancelUnlink,
    confirmUnlink,
  };
}
