import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    setupFiles: './src/test/setup.ts',
    coverage: {
      provider: 'v8',
      reporter: ['text', 'lcov'],
      // ponytail: incremental baseline set just under the measured total at
      // introduction (statements/lines 45%, functions 30%, branches 31%) —
      // raise these as more features gain tests, never lower them.
      thresholds: { lines: 40, statements: 40, functions: 25, branches: 25 },
    },
  },
});
