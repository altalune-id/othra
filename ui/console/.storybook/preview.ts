import type { Preview } from '@storybook/vue3-vite';
import type { App } from 'vue';
import circleFlags from '@iconify-json/circle-flags/icons.json';
import lucide from '@iconify-json/lucide/icons.json';
import { setup } from '@storybook/vue3';
import { addIcons, installStandalone } from '#base/components/ui/standalone';
import '#base/assets/css/main.css';

addIcons(lucide);
addIcons(circleFlags);

setup((app: App) => {
  installStandalone(app);
});

const preview: Preview = {
  globalTypes: {
    theme: {
      description: 'Theme',
      toolbar: { title: 'Theme', icon: 'mirror', items: ['othra-light', 'othra-dark', 'bumblebee-light', 'bumblebee-dark', 'retro-light', 'retro-dark', 'pastel-light', 'pastel-dark', 'caramellatte-light', 'caramellatte-dark', 'coffee-dark', 'luxury-dark', 'night-dark', 'halloween-dark'], dynamicTitle: true },
    },
  },
  initialGlobals: { theme: 'othra-light' },
  loaders: [async () => {
    await Promise.all([document.fonts.load('400 1em "IBM Plex Sans"'), document.fonts.load('400 1em "IBM Plex Mono"')]);
    return {};
  }],
  decorators: [
    (story, context) => {
      document.documentElement.dataset.theme = context.globals.theme as string;
      return { components: { story }, template: '<div class="bg-base-100 p-4 text-base-content"><story /></div>' };
    },
  ],
  parameters: {
    backgrounds: { disable: true },
    controls: {
      matchers: {
        color: /(background|color)$/i,
        date: /Date$/i,
      },
    },
  },
};

export default preview;
