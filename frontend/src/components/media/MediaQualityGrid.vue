<script setup lang="ts">
// 视觉网格容器：视频与音频卡片共用同一套列宽与间距。
</script>

<template>
  <div class="media-grid">
    <slot></slot>
  </div>
</template>

<style scoped>
.media-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 16px;
}
</style>
