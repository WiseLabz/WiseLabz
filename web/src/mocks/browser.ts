import { setupWorker } from 'msw/browser';

import { handlers } from './handlers';

/** MSW worker for the browser. Started by enableMocks() behind USE_MOCKS. */
export const worker = setupWorker(...handlers);
