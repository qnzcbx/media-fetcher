<script setup lang="ts">
import { computed } from "vue";
import { brands, type BrandName, type BrandDefinition } from "../../assets/brands";

const props = withDefaults(defineProps<{
  name: BrandName;
  size?: number | string;
  variant?: "mono" | "color" | "hover";
  strokeWidth?: number;
}>(), { size: 20, variant: "mono" });

const brand = computed<BrandDefinition>(() => brands[props.name]);
const outline = computed(() => brand.value.style === "outline");
</script>

<template>
  <svg
    class="brand-icon"
    :class="`brand-icon--${variant}`"
    :style="{ '--brand-color': brand.color }"
    xmlns="http://www.w3.org/2000/svg"
    :viewBox="brand.viewBox"
    :width="size"
    :height="size"
    :fill="outline ? 'none' : 'currentColor'"
    :stroke="outline ? 'currentColor' : undefined"
    :stroke-width="outline ? strokeWidth ?? brand.defaultStrokeWidth : undefined"
    :stroke-linecap="outline ? 'round' : undefined"
    :stroke-linejoin="outline ? 'round' : undefined"
    aria-hidden="true"
    focusable="false"
  >
    <template v-for="(shape, index) in brand.shapes" :key="index">
      <path v-if="shape.tag === 'path'" v-bind="shape.attrs" />
      <rect v-else v-bind="shape.attrs" />
    </template>
  </svg>
</template>

<style scoped>
.brand-icon { display: inline-block; flex: none; vertical-align: middle; }
.brand-icon--color,
.brand-icon--hover:hover { color: var(--brand-color); }
</style>
