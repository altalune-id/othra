import type { ShownError } from '#base/lib/apperror/message';
import type { Profile } from '#gen/account/v1/account_pb';
import { useErrorMessage } from '#base/composables/useErrorMessage';

// useAccountStore holds the signed-in end user's profile and sign-out form values for the header and the profile screen.
export const useAccountStore = defineStore('account', () => {
  const { $accountClient: client } = useNuxtApp();
  const errorMessage = useErrorMessage();

  const profile = ref<Profile | null>(null);
  const signOutToken = ref('');
  const currentSessionId = ref('');
  const loading = ref(false);
  const loaded = ref(false);
  const loadError = ref<ShownError | null>(null);

  async function load() {
    loading.value = true;
    loadError.value = null;
    try {
      const res = await client.getProfile({});
      profile.value = res.profile ?? null;
      signOutToken.value = res.signOutToken;
      currentSessionId.value = res.currentSessionId;
      loaded.value = true;
    } catch (e) {
      loadError.value = errorMessage(e, 'error.load_profile');
    } finally {
      loading.value = false;
    }
  }

  async function ensureLoaded() {
    if (!loaded.value && !loading.value) {
      await load();
    }
  }

  function replaceProfile(next: Profile | undefined) {
    if (next) {
      profile.value = next;
    }
  }

  return { profile, signOutToken, currentSessionId, loading, loaded, loadError, load, ensureLoaded, replaceProfile };
});
