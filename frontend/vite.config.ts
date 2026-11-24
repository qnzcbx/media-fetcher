import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import UnoCSS from 'unocss/vite'
import path from 'path'
import { fileURLToPath } from 'node:url'

// Vite 的根目录是 frontend，但命令通常在仓库根目录执行，
// 因此显式指定配置文件位置，避免 UnoCSS 在仓库根目录查找失败。
const unoConfig = fileURLToPath(new URL('./uno.config.ts', import.meta.url))

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    vue(),
    UnoCSS({ configFile: unoConfig }),
  ],
  resolve: {
    alias: {
      '@': path.resolve(import.meta.dirname, './src')
    },
  },
  server: {
    port: 5173,
    strictPort: true
  }
})
