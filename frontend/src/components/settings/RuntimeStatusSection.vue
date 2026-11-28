<script setup lang="ts">
import { ref, watch } from "vue";
import { ChevronDown, ChevronUp, RefreshCw } from "@lucide/vue";
import { DesktopService } from "../../mygo";
import {
  formatRuntimeSource,
  formatRuntimeSummary,
  orderedRuntimeItems,
  type RuntimeStatusData,
} from "../../utils/runtimeStatus";

const props = defineProps<{
  open: boolean;
}>();

const loading = ref(false);
const loadError = ref("");
const status = ref<RuntimeStatusData | null>(null);
const showDiagnostics = ref(false);

const loadStatus = async () => {
  if (loading.value) return;
  loading.value = true;
  loadError.value = "";
  try {
    status.value = await DesktopService.getRuntimeStatus();
  } catch (err) {
    loadError.value = err instanceof Error ? err.message : "无法读取运行组件状态";
  } finally {
    loading.value = false;
  }
};

watch(
  () => props.open,
  (isOpen) => {
    if (isOpen) {
      void loadStatus();
    } else {
      showDiagnostics.value = false;
    }
  },
  { immediate: true },
);
</script>

<template>
  <section class="runtime-section" aria-label="运行组件状态">
    <div class="runtime-header">
      <h3>运行组件</h3>
      <button
        type="button"
        class="refresh-btn"
        :disabled="loading"
        aria-label="刷新组件状态"
        @click="loadStatus"
      >
        <RefreshCw :size="14" :class="{ spinning: loading }" />
        <span>刷新</span>
      </button>
    </div>

    <div v-if="loading && !status" class="runtime-hint">正在检测运行组件...</div>
    <div v-else-if="loadError" class="runtime-error">{{ loadError }}</div>

    <div v-else-if="status" class="runtime-list">
      <div
        v-for="item in orderedRuntimeItems(status)"
        :key="item.key"
        class="runtime-row"
        :data-component="item.key"
      >
        <div class="runtime-main">
          <div class="runtime-name-line">
            <span class="status-dot" :class="item.available ? 'ok' : 'bad'" aria-hidden="true"></span>
            <span class="runtime-name">{{ item.name }}</span>
          </div>
          <span class="runtime-version">{{ formatRuntimeSummary(item) }}</span>
        </div>
        <div class="runtime-badges">
          <span class="runtime-badge" :class="item.available ? 'ok' : 'bad'">
            {{ item.available ? "可用" : "异常" }}
          </span>
          <span v-if="item.source !== 'missing'" class="runtime-source">
            {{ formatRuntimeSource(item.source) }}
          </span>
        </div>
      </div>

      <button
        type="button"
        class="diag-toggle"
        :aria-expanded="showDiagnostics"
        @click="showDiagnostics = !showDiagnostics"
      >
        <span>{{ showDiagnostics ? "收起诊断详情" : "查看诊断详情" }}</span>
        <ChevronUp v-if="showDiagnostics" :size="14" />
        <ChevronDown v-else :size="14" />
      </button>

      <div v-if="showDiagnostics" class="diag-panel">
        <div
          v-for="item in orderedRuntimeItems(status)"
          :key="`diag-${item.key}`"
          class="diag-item"
        >
          <div class="diag-title">{{ item.name }}</div>
          <div class="diag-line">状态：{{ item.detail || (item.available ? "正常" : "不可用") }}</div>
          <div v-if="item.path" class="diag-path">路径：{{ item.path }}</div>
          <div v-if="item.pairPath" class="diag-path">配对：{{ item.pairPath }}</div>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.runtime-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.runtime-header h3 {
  margin: 0;
  font-size: 14px;
  font-weight: 500;
  color: var(--text-sub);
}

.refresh-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px 8px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: transparent;
  color: var(--text-sub);
  font-size: 12px;
  cursor: pointer;
}

.refresh-btn:hover:not(:disabled) {
  background: var(--hover-bg);
  color: var(--text-main);
}

.refresh-btn:disabled {
  opacity: 0.6;
  cursor: default;
}

.spinning {
  animation: mf-spin 0.9s linear infinite;
}

@keyframes mf-spin {
  to {
    transform: rotate(360deg);
  }
}

.runtime-hint {
  font-size: 13px;
  color: var(--text-sub);
}

.runtime-error {
  font-size: 13px;
  color: var(--error-text);
  background: var(--error-bg);
  padding: 8px 10px;
  border-radius: 8px;
}

.runtime-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.runtime-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 12px;
  border-radius: 10px;
  background: var(--input-bg);
}

.runtime-main {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.runtime-name-line {
  display: flex;
  align-items: center;
  gap: 7px;
}

.status-dot {
  width: 7px;
  height: 7px;
  border-radius: 999px;
  flex-shrink: 0;
}

.status-dot.ok {
  background: #22c55e;
}

.status-dot.bad {
  background: #ef4444;
}

.runtime-name {
  font-size: 13px;
  font-weight: 500;
  color: var(--text-main);
}

.runtime-version {
  font-size: 12px;
  color: var(--text-sub);
  padding-left: 14px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.runtime-badges {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}

.runtime-badge,
.runtime-source {
  font-size: 11px;
  padding: 2px 7px;
  border-radius: 999px;
  line-height: 1.5;
}

.runtime-badge.ok {
  background: rgba(34, 197, 94, 0.14);
  color: #15803d;
}

.runtime-badge.bad {
  background: rgba(239, 68, 68, 0.14);
  color: #dc2626;
}

.runtime-source {
  background: var(--card-bg);
  border: 1px solid var(--border);
  color: var(--text-sub);
}

.diag-toggle {
  align-self: flex-start;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px 2px;
  border: none;
  background: transparent;
  color: var(--text-sub);
  font-size: 12px;
  cursor: pointer;
}

.diag-toggle:hover {
  color: var(--text-main);
}

.diag-panel {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 10px 12px;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: var(--card-bg);
  font-size: 12px;
}

.diag-title {
  font-weight: 500;
  color: var(--text-main);
  margin-bottom: 2px;
}

.diag-line,
.diag-path {
  color: var(--text-sub);
  word-break: break-all;
  line-height: 1.45;
}
</style>
