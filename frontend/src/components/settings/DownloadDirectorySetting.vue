<script setup lang="ts">
import { ref, watch } from "vue";
import { FolderOpen } from "@lucide/vue";
import { DesktopService } from "../../mygo";

const props = defineProps<{ modelValue: string }>();
const emit = defineEmits<{ "update:modelValue": [value: string] }>();
const path = ref("");
const error = ref("");
const busy = ref(false);
let generation = 0;

watch(
  () => props.modelValue,
  async (value) => {
    const current = ++generation;
    error.value = "";
    path.value = value;
    try {
      const resolved = value || (await DesktopService.getDefaultDownloadDirectory());
      if (current !== generation) return;
      path.value = resolved;
      await DesktopService.validateDownloadDirectory(resolved);
    } catch (cause) {
      if (current === generation)
        error.value = cause instanceof Error ? cause.message : "下载目录不可用，请重新选择";
    }
  },
  { immediate: true },
);

const change = async () => {
  if (busy.value) return;
  busy.value = true;
  error.value = "";
  try {
    const selected = await DesktopService.selectDownloadDirectory(path.value);
    if (selected) emit("update:modelValue", selected);
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : "无法选择下载目录";
  } finally {
    busy.value = false;
  }
};

const open = async () => {
  error.value = "";
  try {
    await DesktopService.openDownloadDirectory(props.modelValue);
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : "无法打开下载目录";
  }
};
</script>

<template>
  <div class="directory-setting">
    <div class="flex-between">
      <span>下载位置</span>
      <button class="directory-icon" aria-label="打开下载目录" @click="open">
        <FolderOpen :size="18" />
      </button>
    </div>
    <span class="directory-mode">{{ modelValue ? "自定义目录" : "系统下载目录" }}</span>
    <p class="directory-path" :title="path">{{ path || "正在读取目录…" }}</p>
    <div class="directory-actions">
      <button class="btn-cancel" :disabled="busy" @click="change">更改</button>
      <button
        class="btn-cancel"
        :disabled="busy || !modelValue"
        @click="emit('update:modelValue', '')"
      >
        恢复默认
      </button>
    </div>
    <p v-if="error" role="alert" class="directory-error">{{ error }}</p>
  </div>
</template>

<style scoped>
.directory-setting {
  margin-top: 24px;
  font-size: 14px;
}
.directory-mode {
  display: block;
  margin-top: 8px;
  color: var(--text-assist);
  font-size: 12px;
}
.directory-path {
  overflow-wrap: anywhere;
  margin: 6px 0 12px;
  line-height: 1.5;
}
.directory-actions {
  @apply flex gap-2;
}
.directory-icon {
  @apply flex items-center border-0 bg-transparent cursor-pointer;
  color: var(--text-main);
  padding: 4px;
  border-radius: 6px;
}
.directory-icon:hover {
  background: var(--hover-bg);
}
.directory-icon:focus-visible {
  outline: 2px solid var(--focus-ring);
}
.directory-error {
  color: var(--error-text);
  font-size: 13px;
}
</style>
