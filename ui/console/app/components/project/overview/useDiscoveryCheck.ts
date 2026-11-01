import type { MaybeRefOrGetter } from 'vue';
import { ref, toValue } from 'vue';
import { discoveryUrls } from '../issuer/discovery';

export type DiscoveryState = 'checking' | 'serving' | 'unreachable';

type Fetch = (url: string) => Promise<{ ok: boolean, json: () => Promise<unknown> }>;

// useDiscoveryCheck fetches the issuer's public discovery document and JWKS from the browser, as an app would.
export function useDiscoveryCheck(issuer: MaybeRefOrGetter<string>, fetcher: Fetch = url => fetch(url)) {
  const state = ref<DiscoveryState>('checking');
  const doc = ref<Record<string, unknown> | null>(null);
  const keys = ref<number | null>(null);

  async function getJSON(url: string): Promise<Record<string, unknown>> {
    const res = await fetcher(url);
    if (!res.ok) {
      throw new Error(url);
    }
    return await res.json() as Record<string, unknown>;
  }

  async function check() {
    const urls = discoveryUrls(toValue(issuer));
    state.value = 'checking';
    keys.value = null;
    try {
      if (!urls) {
        throw new Error('no issuer');
      }
      const found = await getJSON(urls.oidc);
      doc.value = found;
      const jwks = typeof found.jwks_uri === 'string' ? await getJSON(found.jwks_uri) : {};
      keys.value = Array.isArray(jwks.keys) ? jwks.keys.length : 0;
      state.value = 'serving';
    } catch {
      state.value = 'unreachable';
    }
  }

  return { state, doc, keys, check };
}
