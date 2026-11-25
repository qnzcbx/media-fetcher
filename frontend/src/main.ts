import { createApp } from 'vue'
import 'virtual:uno.css'
import './styles/tokens.css'
import './styles/fonts.css'
import './styles/base.css'
import './styles/segmented-control.css'
import App from './App.vue'
import { initTheme } from './composables/usePreferences'

// 首屏渲染前恢复用户主题，避免明显闪烁。
initTheme()

createApp(App).mount('#app')
