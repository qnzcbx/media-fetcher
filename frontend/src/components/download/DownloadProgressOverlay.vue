<script setup lang="ts">
import { computed } from "vue";
import { progressPercent, type GalleryCard } from "../../composables/useDownloads";
import { isProcessingStage, stageLabel } from "../../utils/downloadStatus";
const props = defineProps<{ card: GalleryCard }>();
const processing = computed(() => isProcessingStage(props.card.progress?.stage));
const percent = computed(() => progressPercent(props.card.progress));
const label = computed(() => {
  const { status, progress } = props.card;
  if (status === "failed") return "下载失败";
  if (status === "cancelled") return "已取消";
  if (status === "cancelling") return "取消中";
  if (status === "queued") return "等待下载";
  return stageLabel(progress?.stage);
});
</script>

<template>
  <div v-if="card.status !== 'done'" class="shade" role="status">
    <strong v-if="card.status === 'running' && percent !== undefined">{{ percent }}%</strong>
    <span v-if="card.status === 'running' && processing" class="spinner" aria-hidden="true"></span>
    <progress
      v-if="card.status === 'running' && !processing"
      :value="percent"
      max="100"
      :aria-label="label"
    />
    <span>{{ label }}</span>
  </div>
</template>

<style scoped>
.shade {
  position: absolute;
  inset: 0;
  display: grid;
  align-content: center;
  justify-items: center;
  gap: 8px;
  color: #fff;
  background: #151922a8;
  font-size: 14px;
  pointer-events: none;
}
strong {
  font-size: clamp(24px, 4.4vw, 42px);
  font-weight: 500;
  font-variant-numeric: tabular-nums;
}
progress {
  width: 56%;
  height: 5px;
  accent-color: white;
}
.spinner {
  width: 38px;
  height: 38px;
  border: 3px solid #ffffff35;
  border-top-color: white;
  border-right-color: white;
  border-radius: 50%;
  animation: spin 0.95s linear infinite;
}
@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}
@media (prefers-reduced-motion: reduce) {
  .spinner {
    animation: none;
  }
}
.fade-leave-active {
  transition: opacity 0.22s ease;
}
.fade-leave-to {
  opacity: 0;
}
</style>
