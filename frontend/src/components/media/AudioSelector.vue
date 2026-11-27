<script setup lang="ts">
import { audioKey } from "../../utils/media";
import type { AudioStream } from "../../mygo";

defineProps<{
  audioStreams: AudioStream[];
  modelValue: number;
  selectedAudioKey: string;
}>();

defineEmits<{
  (e: "update:modelValue", val: number, key: string): void;
}>();
</script>

<template>
  <div class="audio-select-container">
    <span>音质</span>
    <select
      class="audio-select"
      :value="selectedAudioKey"
      @change="$emit('update:modelValue', Number(($event.target as HTMLSelectElement).value.split('|')[0]), ($event.target as HTMLSelectElement).value)"
    >
      <option v-for="as in audioStreams" :key="audioKey(as)" :value="audioKey(as)">
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
  min-height: 44px;
  padding: 0 16px;
  background: var(--input-bg);
  border-radius: 8px;
  box-sizing: border-box;
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
