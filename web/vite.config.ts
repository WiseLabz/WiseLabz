import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    proxy: {
      '/api/ws': {
        target: 'ws://localhost:8080',
        ws: true,
      },
      '/api': 'http://localhost:8080',
    },
  },
  build: {
    chunkSizeWarningLimit: 2000, // Increase warning limit to 2MB to avoid warning while maintaining default stable bundling
  },
})
