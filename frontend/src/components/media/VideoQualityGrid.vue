<script setup lang="ts">
import { CheckCircle, LockKeyhole } from "@lucide/vue";

defineProps<{
  videoStreams: any[];
  previewMap: Record<string, string>;
  coverUrl: string;
  selectedVideoID: number;
  selectedVideoCodec: string;
  showUnlockCard?: boolean;
}>();

defineEmits<{
  (e: "select", id: number, codec: string): void;
  (e: "unlock"): void;
}>();
</script>

<template>
  <div class="video-grid">
    <div
      v-if="showUnlockCard"
      class="video-card unlock-card"
      @click="$emit('unlock')"
    >
      <div class="video-cover">
        <img v-if="previewMap[coverUrl]" :src="previewMap[coverUrl]" />
        <div v-else class="preview-placeholder">加载中...</div>
        <div class="unlock-overlay">
          <LockKeyhole :size="28" color="#fff" />
        </div>
      </div>
      <div class="video-quality">点击解锁高品质</div>
    </div>

    <div
      v-for="item in videoStreams"
      :key="item.index"
      class="video-card"
      :class="{
        active: selectedVideoID === item.stream.id && selectedVideoCodec === item.stream.codec,
      }"
      @click="$emit('select', item.stream.id, item.stream.codec)"
    >
      <div class="video-cover">
        <img v-if="previewMap[coverUrl]" :src="previewMap[coverUrl]" />
        <div v-else class="preview-placeholder">加载中...</div>
        <div
          class="check"
          v-if="selectedVideoID === item.stream.id && selectedVideoCodec === item.stream.codec"
        >
          <CheckCircle :size="20" />
        </div>
      </div>
      <div class="video-quality">{{ item.stream.quality_label }}</div>
    </div>
  </div>
</template>

<style scoped>
.video-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 16px;
  margin-bottom: 24px;
}

.video-card {
  background: var(--card-bg);
  border-radius: 8px;
  border: 2px solid transparent;
  cursor: pointer;
  transition: all 0.2s;
  overflow: hidden;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.05);
}

.video-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}

.video-card.active {
  border-color: var(--primary);
}

.video-cover {
  width: 100%;
  aspect-ratio: 16/9;
  background: var(--input-bg);
  position: relative;
}

.video-cover img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.unlock-overlay {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
}

.unlock-card {
  border-color: transparent;
}

.unlock-card .video-quality {
  color: var(--primary);
}

.preview-placeholder {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-sub);
  font-size: 12px;
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

.video-quality {
  padding: 12px;
  text-align: center;
  font-weight: 500;
}
</style>
