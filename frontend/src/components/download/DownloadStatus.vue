<script setup lang="ts">
defineProps<{
  progressInfo: any;
}>();

defineEmits<{
  (e: "cancel"): void;
}>();
</script>

<template>
  <div class="my-downloads" v-if="progressInfo">
    <h3>我的下载 ></h3>
    <div class="download-task">
      <div class="task-header">
        <h4>下载任务</h4>
        <span class="status-badge" :class="progressInfo.stage">{{ progressInfo.stage }}</span>
      </div>
      <div class="task-msg">{{ progressInfo.message }}</div>
      <div
        class="progress-bar-container"
        v-if="progressInfo.stage !== 'done' && progressInfo.stage !== 'error'"
      >
        <div
          class="progress-bar"
          :style="{
            width:
              progressInfo.total > 0
                ? (progressInfo.downloaded / progressInfo.total) * 100 + '%'
                : '100%',
          }"
          :class="{ indeterminate: progressInfo.total <= 0 }"
        ></div>
      </div>
      <div class="task-actions">
        <button
          v-if="progressInfo.stage !== 'done' && progressInfo.stage !== 'error'"
          @click="$emit('cancel')"
          class="btn-cancel"
        >
          取消
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.my-downloads {
  margin-top: 64px;
}

.my-downloads h3 {
  font-size: 16px;
  font-weight: normal;
  color: var(--text-sub);
  margin-bottom: 16px;
}

.download-task {
  background: var(--card-bg);
  border-radius: 12px;
  padding: 20px;
  border: 1px solid var(--border);
}

.task-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.task-header h4 {
  margin: 0;
  font-size: 16px;
}

.status-badge {
  font-size: 12px;
  padding: 4px 8px;
  border-radius: 12px;
  background: var(--input-bg);
  text-transform: capitalize;
}

.status-badge.done {
  background: var(--success-bg);
  color: var(--success-text);
}
.status-badge.error {
  background: var(--error-bg);
  color: var(--error-text);
}
.status-badge.muxing {
  background: var(--warning-bg);
  color: var(--warning-text);
}

.task-msg {
  color: var(--text-sub);
  font-size: 14px;
  margin-bottom: 16px;
}

.progress-bar-container {
  height: 6px;
  background: var(--input-bg);
  border-radius: 3px;
  overflow: hidden;
  margin-bottom: 16px;
}

.progress-bar {
  height: 100%;
  background: var(--primary);
  transition: width 0.3s ease;
}

.progress-bar.indeterminate {
  width: 30%;
  animation: indeterminate 1.5s infinite linear;
}

@keyframes indeterminate {
  0% {
    transform: translateX(-100%);
  }
  100% {
    transform: translateX(330%);
  }
}

.task-actions {
  display: flex;
  justify-content: flex-end;
}
</style>
