import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import App from './App'
import { USE_MOCKS } from './config/env'
import './index.css'

async function bootstrap() {
  // Start mocks (MSW REST + WS emitter) before first render so initial
  // queries/sockets are intercepted. Dynamically imported → tree-shaken out
  // of a prod build with mocks disabled.
  if (USE_MOCKS) {
    const { enableMocks } = await import('./mocks/enable')
    await enableMocks()
  }

  createRoot(document.getElementById('root')!).render(
    <StrictMode>
      <App />
    </StrictMode>,
  )
}

void bootstrap()
