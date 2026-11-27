<script setup lang="ts">
import { SelectRoot, SelectTrigger, SelectValue, SelectPortal, SelectContent, SelectViewport, SelectItem, SelectItemText, SelectItemIndicator } from "reka-ui";
import { ref, computed } from "vue";
import { ChevronDown, Check } from "@lucide/vue";
import { audioKey, sortAudioStreams } from "../../utils/media";
import type { AudioStream } from "../../mygo";

const props = defineProps<{
  audioStreams: AudioStream[];
  modelValue: number;
  selectedAudioKey: string;
}>();

const open = ref(false);
const ordered = computed(() => sortAudioStreams(props.audioStreams));

const emit = defineEmits<{
  (e: "update:modelValue", val: number, key: string): void;
}>();

const handleSelect = (value: unknown) => {
  if (!value) return;
  const key = String(value);
  const found = props.audioStreams.find((audio) => audioKey(audio) === key);
  emit("update:modelValue", found ? found.id : Number(key.split("|")[0]), key);
};
</script>

<template>
  <div class="audio-select-container">
    <span>音质</span>
    <SelectRoot v-model:open="open" :modelValue="selectedAudioKey" @update:modelValue="handleSelect">
      <SelectTrigger class="audio-select" aria-label="音质"><SelectValue /><ChevronDown :size="16" class="audio-chevron" :class="{ open }" /></SelectTrigger>
      <SelectPortal><SelectContent class="audio-menu" position="popper" align="end" :sideOffset="6" :collisionPadding="12"><SelectViewport>
        <SelectItem v-for="audio in ordered" :key="audioKey(audio)" :value="audioKey(audio)" class="audio-item"><SelectItemText>{{ audio.quality }}</SelectItemText><SelectItemIndicator class="audio-check"><Check :size="15" /></SelectItemIndicator></SelectItem>
      </SelectViewport></SelectContent></SelectPortal>
    </SelectRoot>
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

:global(.audio-menu) { z-index:1101; background:var(--card-bg); color:var(--text-main); padding:4px; border:1px solid var(--border); border-radius:8px; max-height:var(--reka-select-content-available-height); overflow:auto; min-width:180px; max-width:calc(100vw - 24px); box-shadow:0 6px 20px var(--shadow-color); }
:global(.audio-item) { display:flex; align-items:center; justify-content:space-between; gap:16px; padding:8px 12px; border-radius:4px; text-align:left; cursor:pointer; outline:none; }
:global(.audio-item[data-state="checked"]) { color:var(--selection-text); background:var(--selection-bg); }
:global(.audio-item[data-highlighted]) { background:var(--input-bg); }
.audio-select {
  border: none;
  background: transparent;
  font-size: 14px;
  outline: none;
  margin-left: auto;
  display: inline-flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  padding: 0;
  min-width: 0;
  text-align: right;
  color: var(--text-main);
  cursor: pointer;
}
.audio-chevron { flex-shrink:0; transition:transform 150ms ease; }
.audio-chevron.open { transform:rotate(180deg); }
.audio-check { display:inline-flex; }
</style>
