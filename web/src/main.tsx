import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import App from './App'
import { USE_MOCKS } from './config/env'
import './index.css'
import './i18n'
import { useTheme } from './store/theme'

// Initializing the theme store applies the persisted palette + fonts to :root
// before first paint (see src/theme.ts + Settings → Theme).
useTheme.getState()

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
