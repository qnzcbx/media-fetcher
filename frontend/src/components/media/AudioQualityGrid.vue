<script setup lang="ts">
import { audioKey, sortAudioStreams } from "../../utils/media";
import { computed } from "vue";
import MediaQualityCard from "./MediaQualityCard.vue";
import MediaQualityGrid from "./MediaQualityGrid.vue";
import type { AudioStream } from "../../mygo";

const props = defineProps<{
  audioStreams: AudioStream[];
  cover?: string;
  selectedAudioID: number;
  selectedAudioKey: string;
}>();

const emit = defineEmits<{
  (e: "select", id: number, key: string): void;
}>();

// 展示顺序与音质菜单保持一致，不影响默认选择。
const ordered = computed(() => sortAudioStreams(props.audioStreams));
</script>

<template>
  <MediaQualityGrid>
    <MediaQualityCard
      v-for="stream in ordered"
      :key="audioKey(stream)"
      :cover="cover"
      :label="stream.quality"
      kind="audio"
      :selected="selectedAudioKey === audioKey(stream)"
      @select="emit('select', stream.id, audioKey(stream))"
    />
  </MediaQualityGrid>
</template>
