<script setup lang="ts">
import { ref, watch } from "vue";
import { CheckCircle, LockKeyhole, Music } from "@lucide/vue";

// 视频/音频共用的媒体质量卡片：只负责展示与点击，不感知具体业务 ID。
const props = withDefaults(
  defineProps<{
    cover?: string;
    label: string;
    selected?: boolean;
    kind?: "video" | "audio";
    locked?: boolean;
    accentLabel?: boolean;
  }>(),
  {
    cover: "",
    selected: false,
    kind: "video",
    locked: false,
    accentLabel: false,
  },
);

const emit = defineEmits<{
  (e: "select"): void;
}>();

const failed = ref(false);
watch(
  () => props.cover,
  () => {
    failed.value = false;
  },
);
</script>

<template>
  <div
    class="media-card"
    :class="{ active: selected, locked, 'accent-label': accentLabel }"
    role="button"
    tabindex="0"
    @click="emit('select')"
    @keydown.enter.prevent="emit('select')"
    @keydown.space.prevent="emit('select')"
  >
    <div class="media-cover">
      <img v-if="cover && !failed" :src="cover" alt="" @error="failed = true" />
      <div v-else class="cover-placeholder">{{ cover ? "暂无封面" : "加载中..." }}</div>
      <Music v-if="kind === 'audio' && !locked" class="kind-badge" :size="22" aria-hidden="true" />
      <div v-if="locked" class="lock-overlay">
        <LockKeyhole :size="28" color="#fff" />
      </div>
      <div v-else-if="selected" class="check">
        <CheckCircle :size="20" />
      </div>
    </div>
    <div class="media-label">{{ label }}</div>
  </div>
</template>

<style scoped>
.media-card {
  background: var(--card-bg);
  border-radius: 8px;
  border: 2px solid transparent;
  cursor: pointer;
  transition: all 0.2s;
  overflow: hidden;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.05);
}

.media-card:hover {
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}

.media-card:focus-visible {
  outline: 2px solid var(--primary);
  outline-offset: 2px;
}

.media-card.active {
  border-color: var(--primary);
}

.media-cover {
  width: 100%;
  aspect-ratio: 16/9;
  background: var(--input-bg);
  position: relative;
}

.media-cover img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.cover-placeholder {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-sub);
  font-size: 12px;
}

.kind-badge {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  width: 38px;
  height: 38px;
  padding: 8px;
  border-radius: 50%;
  background: rgba(19, 21, 26, 0.45);
  color: #fff;
  box-sizing: content-box;
}

.lock-overlay {
  position: absolute;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
}

.check {
  position: absolute;
  top: 8px;
  right: 8px;
  color: var(--primary);
  background: #fff;
  border-radius: 50%;
  display: flex;
}

.media-label {
  padding: 12px;
  min-height: 20px;
  text-align: center;
  font-weight: 500;
}

.media-card.accent-label .media-label {
  color: var(--primary);
}
</style>
