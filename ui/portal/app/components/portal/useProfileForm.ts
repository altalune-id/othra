import type { ShownError } from '#base/lib/apperror/message';
import { create } from '@bufbuild/protobuf';
import { useValidation } from '#base/composables/useValidation';
import { UpdateProfileRequestSchema } from '#gen/account/v1/account_pb';
import { useAccountStore } from '~/stores/account';

// useProfileForm edits the end user's name, starting from the stored profile and writing the saved one back.
export function useProfileForm() {
  const { $accountClient: client } = useNuxtApp();
  const account = useAccountStore();
  const validation = useValidation(UpdateProfileRequestSchema, { fields: ['name'] });
  const name = ref(account.profile?.name ?? '');
  const busy = ref(false);
  const saved = ref(false);
  const error = ref<ShownError | null>(null);

  watch(() => account.profile?.name, (next) => {
    name.value = next ?? '';
  });
  watch(name, (next) => {
    if (next.trim() !== (account.profile?.name ?? '')) {
      saved.value = false;
    }
  });

  async function save(): Promise<boolean> {
    if (busy.value) {
      return false;
    }
    saved.value = false;
    error.value = null;
    const req = { name: name.value.trim() };
    if (!validation.check(create(UpdateProfileRequestSchema, req))) {
      error.value = validation.formError.value;
      return false;
    }
    busy.value = true;
    try {
      account.replaceProfile((await client.updateProfile(req)).profile);
      saved.value = true;
      return true;
    } catch (e) {
      error.value = validation.fromServer(e, 'error.save_profile');
      return false;
    } finally {
      busy.value = false;
    }
  }

  return { name, busy, saved, error, fieldErrors: validation.errors, save };
}
