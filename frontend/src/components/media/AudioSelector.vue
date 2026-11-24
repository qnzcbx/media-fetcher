<script setup lang="ts">
defineProps<{
  audioStreams: any[];
  modelValue: number;
}>();

defineEmits<{
  (e: "update:modelValue", val: number): void;
}>();
</script>

<template>
  <div class="audio-select-container">
    <span>音质</span>
    <select
      class="audio-select"
      :value="modelValue"
      @change="$emit('update:modelValue', Number(($event.target as HTMLSelectElement).value))"
    >
      <option v-for="as in audioStreams" :key="as.id" :value="as.id">
        {{ as.quality }}
      </option>
    </select>
  </div>
</template>

<style scoped>
.audio-select-container {
  display: flex;
  align-items: center;
  gap: 12px;
  background: var(--input-bg);
  padding: 12px 16px;
  border-radius: 8px;
}

.audio-select {
  border: none;
  background: transparent;
  font-size: 14px;
  outline: none;
  flex: 1;
  color: var(--text-main);
  cursor: pointer;
}
</style>
