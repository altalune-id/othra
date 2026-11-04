import { siblingURL } from '~/lib/auth/urls';
import { useAuthStore } from '~/stores/auth';

// useUserMenu gives the account menu its signed-in person, sign-out URL and translated labels, wherever the shell shows it.
export function useUserMenu() {
  const { t, locale, locales, setLocale } = useI18n();
  const auth = useAuthStore();
  const { apiUrl, appVersion } = useRuntimeConfig().public;
  const labels = computed(() => ({
    signOut: t('auth.sign_out'),
    signedInAs: t('auth.signed_in_as'),
    operator: t('auth.operator'),
    apikey: t('auth.source_apikey'),
    basic: t('auth.source_basic'),
    profile: t('profile.title'),
    apiKeys: t('apikeys.personal_title'),
    language: t('language.label'),
  }));
  const languages = computed(() => locales.value.map(l => (typeof l === 'string' ? { code: l, name: l } : { code: l.code, name: l.name ?? l.code })));
  return { me: computed(() => auth.me), logoutUrl: siblingURL(apiUrl, '/logout'), labels, version: appVersion, locale, languages, setLocale: (code: string) => setLocale(code as typeof locale.value) };
}
