<script setup lang="ts">
import MediaQualityCard from "./MediaQualityCard.vue";
import MediaQualityGrid from "./MediaQualityGrid.vue";
import { pickVideoStreamForGroup, type UniqueVideoStream } from "../../utils/media";

const props = defineProps<{
  videoStreams: UniqueVideoStream[];
  previewMap: Record<string, string>;
  coverUrl: string;
  selectedVideoID: number;
  selectedVideoGroupKey?: string;
  selectedVideoCodec: string;
  showUnlockCard?: boolean;
}>();

const emit = defineEmits<{
  (e: "select", id: number, codec: string, groupKey: string, streamKey: string): void;
  (e: "unlock"): void;
}>();

const selectQuality = (item: UniqueVideoStream) => {
  const target = pickVideoStreamForGroup(item, props.selectedVideoCodec);
  emit("select", target.id, target.codec, item.groupKey, target.stream_key ?? "");
};
</script>

<template>
  <MediaQualityGrid>
    <MediaQualityCard
      v-if="showUnlockCard"
      :cover="previewMap[coverUrl]"
      label="点击解锁高品质"
      locked
      accent-label
      @select="emit('unlock')"
    />

    <MediaQualityCard
      v-for="item in videoStreams"
      :key="item.groupKey"
      :cover="previewMap[coverUrl]"
      :label="item.stream.quality_label"
      :selected="selectedVideoGroupKey ? selectedVideoGroupKey === item.groupKey : selectedVideoID === item.stream.id"
      @select="selectQuality(item)"
    />
  </MediaQualityGrid>
</template>
