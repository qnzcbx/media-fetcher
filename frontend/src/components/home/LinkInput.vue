<script setup lang="ts">
import { ref, watch } from "vue";
import { Download, ClipboardPaste, X } from "@lucide/vue";

const props = defineProps<{
  modelValue: string;
  isParsing: boolean;
}>();

const emit = defineEmits<{
  (e: "update:modelValue", value: string): void;
  (e: "submit"): void;
  (e: "pasteError", message: string): void;
}>();

const localUrl = ref(props.modelValue);

watch(
  () => props.modelValue,
  (newVal) => {
    localUrl.value = newVal;
  }
);

watch(localUrl, (newVal) => {
  emit("update:modelValue", newVal);
});

const onPaste = async () => {
  try {
    const text = await navigator.clipboard.readText();
    if (text) {
      localUrl.value = text;
    }
  } catch (err) {
    emit("pasteError", "无法读取剪贴板，请手动粘贴");
  }
};
</script>

<template>
  <form class="search-box" @submit.prevent="$emit('submit')">
    <input v-model="localUrl" type="text" placeholder="你的链接" :disabled="isParsing" required />
    <div class="icon-btn-box">
      <button
        v-if="localUrl"
        type="button"
        @click="localUrl = ''"
        class="icon-btn-small"
        title="清空"
        :disabled="isParsing"
      >
        <X class="icon" />
      </button>
      <button
        v-else
        type="button"
        @click="onPaste"
        class="icon-btn-small"
        title="粘贴"
        :disabled="isParsing"
      >
        <ClipboardPaste class="icon" />
      </button>
      <button type="submit" class="icon-btn-small primary" title="下载" :disabled="isParsing">
        <Download class="icon" />
      </button>
    </div>
  </form>
</template>

<style scoped>
.search-box {
  display: flex;
  background: var(--input-bg);
  border-radius: 12px;
  padding: 8px 16px;
  align-items: center;
  border: 1px solid transparent;
  transition: border-color 0.3s, background 0.3s;
}

.search-box:focus-within {
  background: var(--card-bg);
  border-color: var(--primary);
  box-shadow: 0 4px 16px var(--selection-soft);
}

.search-box input {
  flex: 1;
  border: none;
  outline: none;
  font-size: 16px;
  background: transparent;
  padding: 8px 0;
}

.icon-btn-box {
  display: flex;
  gap: 6px;
}

.icon-btn-small {
  background: transparent;
  border: none;
  color: var(--text-sub);
  padding: 8px;
  cursor: pointer;
  border-radius: 6px;
  display: flex;
  align-items: center;
  transition: all 0.2s;
}

.icon-btn-small:hover:not(:disabled) {
  background: var(--hover-bg);
  color: var(--text-main);
}

.icon-btn-small.primary {
  color: var(--primary-hover);
}

.icon-btn-small.primary:hover:not(:disabled) {
  background: var(--primary);
  color: var(--primary-text);
}
</style>
