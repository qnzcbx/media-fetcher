<script setup lang="ts">
import { computed } from "vue";
import MediaQualityCard from "./MediaQualityCard.vue";
import MediaQualityGrid from "./MediaQualityGrid.vue";
import type { AudioStream } from "../../mygo";

const props = defineProps<{
  audioStreams: AudioStream[];
  cover?: string;
  selectedAudioID: number;
}>();

const emit = defineEmits<{
  (e: "select", id: number): void;
}>();

// 按真实带宽倒序展示，默认项由上层选择逻辑给出。
const ordered = computed(() =>
  [...(props.audioStreams ?? [])].sort((a, b) => b.bandwidth - a.bandwidth),
);
</script>

<template>
  <MediaQualityGrid>
    <MediaQualityCard
      v-for="stream in ordered"
      :key="stream.id"
      :cover="cover"
      :label="stream.quality"
      kind="audio"
      :selected="selectedAudioID === stream.id"
      @select="emit('select', stream.id)"
    />
  </MediaQualityGrid>
</template>
