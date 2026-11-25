<script setup lang="ts">
import BrandIcon from "../icons/BrandIcon.vue";
</script>

<template>
  <div class="support-text">
    <span class="brand-hover"><BrandIcon name="bilibili" variant="hover" /> Bilibili</span>
    <span class="brand-hover"><BrandIcon name="lofter" variant="hover" /> LOFTER</span>
  </div>
</template>

<style scoped>
.support-text {
  @apply flex items-center justify-center;
  gap: 16px;
  color: var(--text-assist);
  font-size: 14px;
  margin-top: 16px;
  letter-spacing: 1px;
}

.brand-hover {
  @apply inline-flex items-center;
  gap: 5px;
}
.brand-hover:hover :deep(.brand-icon--hover) { color: var(--brand-color); }
</style>
