<script setup lang="ts">
import { ref, useId } from "vue";
import { Check } from "@lucide/vue";
import type { DeletePlan } from "../../mygo";
import {
  CheckboxRoot,
  CheckboxIndicator,
  DialogRoot,
  DialogPortal,
  DialogOverlay,
  DialogContent,
  DialogTitle,
  DialogDescription,
} from "reka-ui";

defineProps<{
  confirmation?: {
    ids: string[];
    names: string[];
    skipped: number;
    plan?: DeletePlan;
    deleteFiles: boolean;
  };
  busy: boolean;
  error: string;
}>();
const emit = defineEmits<{ cancel: []; confirm: [] }>();
const cancel = ref<HTMLButtonElement>();
const checkboxId = useId();
</script>

<template>
  <DialogRoot :open="!!confirmation" @update:open="(open) => !open && !busy && emit('cancel')">
    <DialogPortal>
      <DialogOverlay class="delete-overlay" />
      <DialogContent
        class="delete-dialog"
        @open-auto-focus.prevent="cancel?.focus()"
        @escape-key-down.stop
      >
        <DialogTitle>删除</DialogTitle>
        <p class="filename">
          {{
            confirmation?.names.length === 1
              ? confirmation.names[0]
              : "已选择 " + (confirmation?.ids.length ?? 0) + " 项"
          }}
        </p>
        <div class="file-option">
          <CheckboxRoot
            :id="checkboxId"
            class="file-checkbox"
            :model-value="confirmation?.deleteFiles ?? false"
            :disabled="busy || !confirmation?.plan?.targets.length"
            @update:model-value="
              (value) => {
                if (confirmation) confirmation.deleteFiles = value === true;
              }
            "
          >
            <CheckboxIndicator class="checkbox-indicator">
              <Check :size="14" :stroke-width="2.5" />
            </CheckboxIndicator>
          </CheckboxRoot>
          <label :for="checkboxId">同时删除本地文件</label>
        </div>

        <DialogDescription class="delete-hint">
          {{ confirmation?.deleteFiles ? "文件将移至回收站。" : "仅移除下载记录，保留本地文件。" }}
        </DialogDescription>
        <p v-if="confirmation?.deleteFiles && confirmation.skipped">
          跳过 {{ confirmation.skipped }} 项不可回收的记录。
        </p>
        <p v-if="error" role="alert">{{ error }}</p>
        <div class="buttons">
          <button ref="cancel" class="btn-cancel" :disabled="busy" @click="emit('cancel')">
            取消
          </button>
          <button class="delete" :disabled="busy" @click="emit('confirm')">删除</button>
        </div>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>

<style scoped>
.delete-overlay {
  position: fixed;
  inset: 0;
  background: var(--overlay-bg);
  z-index: var(--layer-confirm-overlay);
}

.delete-dialog {
  position: fixed;
  left: 50%;
  top: 50%;
  transform: translate(-50%, -50%);
  background: var(--card-bg);
  color: var(--text-main);
  border-radius: 14px;
  padding: 24px;
  width: min(360px, calc(100vw - 64px));
  max-height: 80vh;
  overflow: auto;
  z-index: var(--layer-confirm);
}

.filename {
  overflow-wrap: anywhere;
}

.file-option {
  display: flex;
  align-items: center;
  gap: 8px;
}

.file-checkbox {
  display: grid;
  place-items: center;
  width: 18px;
  height: 18px;
  padding: 0;
  border: 1px solid var(--border);
  border-radius: 4px;
  background: var(--input-bg);
  color: var(--selection-text);
  cursor: pointer;
}

.file-checkbox[data-state="checked"] {
  background: var(--selection-bg);
  border-color: var(--primary);
}

.file-checkbox:focus-visible {
  outline: 2px solid var(--focus-ring);
  outline-offset: 2px;
}

.file-checkbox:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.checkbox-indicator {
  display: flex;
}

.delete-hint {
  color: var(--text-assist);
  font-size: 13px;
  margin: 8px 0 0 26px;
}

.buttons {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  margin-top: 22px;
}

.delete {
  border: 0;
  border-radius: 6px;
  padding: 8px 16px;
  background: var(--danger);
  color: white;
  cursor: pointer;
}
</style>
