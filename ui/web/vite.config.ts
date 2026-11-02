import { defineConfig } from 'vite';

export default defineConfig({
  build: {
    lib: {
      entry: { password: 'src/password/index.ts', lang: 'src/lang/index.ts' },
      formats: ['es'],
      fileName: (_format, name) => `${name}.js`,
    },
    outDir: '../../internal/web/static',
    emptyOutDir: false,
    minify: true,
  },
});
