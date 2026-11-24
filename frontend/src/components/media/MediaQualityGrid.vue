<script setup lang="ts">
// 视觉网格容器：视频与音频卡片共用同一套列宽与间距。
</script>

<template>
  <div class="media-grid grid gap-4">
    <slot></slot>
  </div>
</template>

<style scoped>
/* 响应式列宽属于复杂布局规则，保留原生 CSS。 */
.media-grid {
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
}
</style>
