import {
  defineConfig,
  presetWind3,
  transformerDirectives,
  transformerVariantGroup,
} from 'unocss'

// 颜色仅映射 tokens.css 中的设计变量，避免出现第二套色值来源。
export default defineConfig({
  presets: [presetWind3()],

  // @apply 与 hover:(...) 等写法由 transformerDirectives / transformerVariantGroup 完成转换。
  transformers: [transformerDirectives(), transformerVariantGroup()],

  theme: {
    colors: {
      primary: 'var(--primary)',
      'primary-hover': 'var(--primary-hover)',
      surface: 'var(--card-bg)',
      canvas: 'var(--bg-color)',
      field: 'var(--input-bg)',
      main: 'var(--text-main)',
      muted: 'var(--text-sub)',
      outline: 'var(--border)',
    },
  },

  shortcuts: {
    'flex-center': 'flex items-center justify-center',
    'flex-between': 'flex items-center justify-between',
    'flex-fill': 'flex-1 min-h-0 min-w-0',
  },
})
