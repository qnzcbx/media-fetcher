<script setup lang="ts">
import MediaQualityCard from "./MediaQualityCard.vue";
import MediaQualityGrid from "./MediaQualityGrid.vue";
import type { UniqueVideoStream } from "../../utils/media";

defineProps<{
  videoStreams: UniqueVideoStream[];
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
  <MediaQualityGrid>
    <MediaQualityCard
      v-if="showUnlockCard"
      :cover="previewMap[coverUrl]"
      label="点击解锁高品质"
      locked
      accent-label
      @select="$emit('unlock')"
    />

    <MediaQualityCard
      v-for="item in videoStreams"
      :key="item.index"
      :cover="previewMap[coverUrl]"
      :label="item.stream.quality_label"
      :selected="selectedVideoID === item.stream.id && selectedVideoCodec === item.stream.codec"
      @select="$emit('select', item.stream.id, item.stream.codec)"
    />
  </MediaQualityGrid>
</template>
