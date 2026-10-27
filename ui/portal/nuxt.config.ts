export default defineNuxtConfig({
  extends: ['@othra/base'],
  compatibilityDate: '2026-10-06',
  components: { dirs: [] },
  ssr: false,
  icon: {
    clientBundle: { icons: ['lucide:user-round', 'lucide:log-in', 'lucide:monitor-smartphone', 'lucide:blocks', 'lucide:key-round'] },
  },
  i18n: {
    locales: [
      { code: 'en-US', file: 'en-US.json' },
      { code: 'id-ID', file: 'id-ID.json' },
    ],
  },
  app: {
    baseURL: '/',
    cdnURL: '/portal/',
    head: { link: [{ key: 'icon', rel: 'icon', href: 'data:,' }] },
  },
  runtimeConfig: {
    public: { apiUrl: '', handle: '', project: '', logoUrl: '', termsUrl: '', privacyUrl: '', hostedUrl: '' },
  },
});
