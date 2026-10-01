import { defineConfig } from 'orval';

export default defineConfig({
  wiselabz: {
    input: {
      target: '../docs/openapi.yaml',
    },
    output: {
      mode: 'tags-split',
      target: 'src/api/generated',
      schemas: 'src/api/model',
      client: 'react-query',
      httpClient: 'axios',
      // Generate MSW handlers from the same spec. Toggle off once the backend is live.
      mock: true,
      clean: true,
      override: {
        // Route every generated call through our axios instance (baseURL + auth).
        mutator: {
          path: 'src/api/axios-instance.ts',
          name: 'customInstance',
        },
        query: {
          // useQuery stays at its default (GET only); forcing it on would turn
          // body-less POSTs like postSync into queries instead of mutations.
          // Changes/alerts feeds are paginated; opt specific keys into infinite later.
          useInfinite: false,
        },
        // Deterministic-ish mock data; bump count for list endpoints.
        mock: {
          arrayMin: 1,
          arrayMax: 4,
        },
      },
    },
    hooks: {
      // Format generated output so it passes the repo's eslint/prettier gate.
      afterAllFilesWrite: 'prettier --write',
    },
  },
});
