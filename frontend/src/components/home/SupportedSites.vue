<script setup lang="ts">
import { brands } from "../../assets/brands";
import BrandIcon from "../icons/BrandIcon.vue";
</script>

<template>
  <div class="support-text">
    <span class="brand-hover" :style="{ '--brand-color': brands.bilibili.color }"><BrandIcon name="bilibili" /> Bilibili</span>
    <span class="brand-hover" :style="{ '--brand-color': brands.youtube.color }"><BrandIcon name="youtube" /> YouTube</span>
    <span class="brand-hover" :style="{ '--brand-color': brands.lofter.color }"><BrandIcon name="lofter" /> LOFTER</span>
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
  transition: color 160ms ease;
}
.brand-hover:hover { color: var(--brand-color); }
</style>
