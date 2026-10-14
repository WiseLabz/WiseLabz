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
      // Floors sit a few points under the measured totals (statements 67.7%,
      // lines 68.6%, branches 61.8%, functions 52.7%) so an unrelated refactor
      // does not fail the gate. Raise them as coverage grows, never lower them.
      thresholds: { lines: 65, statements: 64, functions: 49, branches: 58 },
    },
  },
});
