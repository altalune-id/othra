import vue from '@othra/vitest-config/vue';
import AutoImport from 'unplugin-auto-import/vite';
import { defineConfig, mergeConfig } from 'vitest/config';
import { baseAliases } from '../base/aliases';

export default mergeConfig(vue, defineConfig({
  plugins: [AutoImport({ imports: ['vue'], dts: false })],
  resolve: { alias: baseAliases },
}));
