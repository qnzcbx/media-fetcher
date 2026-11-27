<script setup lang="ts">
import { audioKey } from "../../utils/media";
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

// 按真实带宽倒序展示，默认项由上层选择逻辑给出。
const ordered = computed(() =>
  [...(props.audioStreams ?? [])].sort((a, b) => b.bandwidth - a.bandwidth),
);
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
