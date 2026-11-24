<script setup lang="ts">
import { Check, Monitor, Moon, Sun } from "@lucide/vue";
import type { ThemePreference } from "../../utils/preferences";

defineProps<{
  modelValue: ThemePreference;
}>();

const emit = defineEmits<{
  (e: "update:modelValue", value: ThemePreference): void;
}>();

const options = [
  { id: "light", label: "浅色", icon: Sun },
  { id: "dark", label: "深色", icon: Moon },
  { id: "system", label: "跟随系统", icon: Monitor },
] as const;
</script>

<template>
  <div class="theme-options" role="radiogroup" aria-label="外观">
    <button
      v-for="option in options"
      :key="option.id"
      type="button"
      role="radio"
      class="theme-option"
      :class="{ active: modelValue === option.id }"
      :aria-checked="modelValue === option.id"
      @click="emit('update:modelValue', option.id)"
    >
      <component :is="option.icon" :size="18" class="theme-icon" aria-hidden="true" />
      <span>{{ option.label }}</span>
      <Check v-if="modelValue === option.id" :size="16" class="theme-check" aria-hidden="true" />
    </button>
  </div>
</template>

<style scoped>
.theme-options {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.theme-option {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
  padding: 11px 14px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: transparent;
  color: var(--text-main);
  font: inherit;
  font-size: 14px;
  text-align: left;
  cursor: pointer;
  transition: background-color 0.18s ease, border-color 0.18s ease, color 0.18s ease;
}

.theme-option:hover {
  background: var(--hover-bg);
}

.theme-option:focus-visible {
  outline: 2px solid var(--focus-ring);
  outline-offset: 1px;
}

.theme-option.active {
  background: var(--selection-bg);
  border-color: var(--selection-bg);
  color: var(--selection-text);
}

.theme-icon {
  flex-shrink: 0;
}

.theme-check {
  margin-left: auto;
}
</style>
