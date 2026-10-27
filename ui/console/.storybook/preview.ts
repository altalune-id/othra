import type { Preview } from '@storybook/vue3-vite';
import type { App } from 'vue';
import circleFlags from '@iconify-json/circle-flags/icons.json';
import lucide from '@iconify-json/lucide/icons.json';
import { addCollection } from '@iconify/vue/offline';
import { setup } from '@storybook/vue3';
import { installStandalone } from '#base/components/ui/standalone';
import '#base/assets/css/main.css';

addCollection(lucide);
addCollection(circleFlags);

setup((app: App) => {
  installStandalone(app);
});

const preview: Preview = {
  parameters: {
    controls: {
      matchers: {
        color: /(background|color)$/i,
        date: /Date$/i,
      },
    },
  },
};

export default preview;
