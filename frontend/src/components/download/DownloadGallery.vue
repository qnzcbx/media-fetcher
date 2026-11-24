<script setup lang="ts">
import { ChevronRight, Grid2X2 } from "@lucide/vue";
import type { GalleryCard } from "../../composables/useDownloads";
import DownloadCard from "./DownloadCard.vue";

const props = defineProps<{ cards: GalleryCard[]; busy: boolean; error: string }>();
const emit = defineEmits<{
  action: [card: GalleryCard, action: string];
  cancel: [];
  openDrawer: [];
  openView: [];
}>();
</script>

<template>
  <section v-if="cards.length > 0" class="gallery" aria-label="我的下载">
    <header>
      <h2>我的下载</h2>
      <div class="actions">
        <button class="icon-btn" aria-label="打开快捷图库" @click="emit('openDrawer')">
          <Grid2X2 :size="16" />
        </button>
        <button class="icon-btn" aria-label="打开详细管理页" @click="emit('openView')">
          <ChevronRight :size="18" />
        </button>
      </div>
    </header>
    <p v-if="error" role="alert" class="error-text">{{ error }}</p>
    <div class="grid recent">
      <DownloadCard
        v-for="card in cards.slice(0, 4)"
        :key="card.id"
        :card="card"
        :busy="busy"
        @action="(item, action) => emit('action', item, action)"
        @cancel="emit('cancel')"
      />
    </div>
  </section>
</template>

<style scoped>
.gallery { width:calc(100vw - 36px); max-width:1160px; align-self:center; margin-top:clamp(48px, 8vh, 82px); text-align:left; }
header { display:flex; align-items:center; justify-content:space-between; margin-bottom:14px; }
h2 { font-size:clamp(18px, 2.4vw, 24px); margin:0; font-weight:500; }
.actions { display:flex; gap:16px; align-items:center; }
.icon-btn { display:flex; align-items:center; border:0; background:transparent; color:var(--text-sub); cursor:pointer; padding: 4px; }
.icon-btn:hover { color: var(--primary); }
.grid { display:grid; grid-template-columns:repeat(4,minmax(0,1fr)); gap:10px; }
.error-text { color:var(--danger); font-size:13px; margin-bottom: 8px; }
@media (max-width:460px) { .grid { grid-template-columns:repeat(3,minmax(0,1fr)); } .recent > :nth-child(n+4) { display:none; } }
</style>
