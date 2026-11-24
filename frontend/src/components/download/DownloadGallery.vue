<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { ChevronRight, Grid2X2, X } from "@lucide/vue";
import type { GalleryCard } from "../../composables/useDownloads";
import DownloadCard from "./DownloadCard.vue";

const props = defineProps<{ cards: GalleryCard[]; busy: boolean; error: string }>();
const emit = defineEmits<{ action: [card: GalleryCard, action: string]; cancel: [] }>();
const open = ref(false);
const filter = ref("");
const page = ref(0);
watch(filter, () => { page.value = 0; });
const filters = computed(() => [
  { id: "", name: "全部" }, { id: "video", name: "视频" }, { id: "image", name: "图片" },
  ...(props.cards.some((item) => item.kind === "audio") ? [{ id: "audio", name: "音频" }] : []),
]);
const filtered = computed(() => props.cards.filter((item) => !filter.value || item.kind === filter.value));
watch(() => filtered.value.length, (length) => { page.value = Math.min(page.value, Math.max(0, Math.ceil(length / 25) - 1)); });
</script>

<template>
  <section class="gallery" aria-label="我的下载">
    <header><h2>我的下载</h2><button class="icon-btn" aria-label="打开完整图库" @click="open = true"><Grid2X2 :size="16" /><ChevronRight :size="18" /></button></header>
    <p v-if="error" role="alert">{{ error }}</p>
    <div v-if="cards.length" class="grid recent">
      <DownloadCard v-for="card in cards.slice(0, 4)" :key="card.id" :card="card" :busy="busy" @action="(item, action) => emit('action', item, action)" @cancel="emit('cancel')" />
    </div>
    <Teleport to="body">
      <div v-if="open" class="modal-overlay" @click.self="open = false" @keydown.esc="open = false">
        <section class="modal full" role="dialog" aria-modal="true" aria-label="我的下载">
          <header><h2>我的下载</h2><button class="icon-btn" aria-label="关闭" @click="open = false"><X :size="20" /></button></header>
          <p v-if="error" role="alert">{{ error }}</p>
          <nav aria-label="资源类型">
            <button v-for="item in filters" :key="item.id" :aria-pressed="filter === item.id" @click="filter = item.id">{{ item.name }}</button>
          </nav>
          <div class="grid">
            <DownloadCard v-for="card in filtered.slice(page * 25, (page + 1) * 25)" :key="card.id" :card="card" :busy="busy" @action="(item, action) => emit('action', item, action)" @cancel="emit('cancel')" />
          </div>
          <nav v-if="filtered.length > 25" aria-label="图库分页">
            <button :disabled="page === 0" @click="page--">上一页</button>
            <button :disabled="(page + 1) * 25 >= filtered.length" @click="page++">下一页</button>
          </nav>
        </section>
      </div>
    </Teleport>
  </section>
</template>

<style scoped>
.gallery { width:calc(100vw - 36px); max-width:1160px; align-self:center; margin-top:clamp(48px, 8vh, 82px); text-align:left; }
header { display:flex; align-items:center; justify-content:space-between; margin-bottom:14px; }
h2 { font-size:clamp(18px, 2.4vw, 24px); margin:0; font-weight:500; }
header .icon-btn { display:flex; align-items:center; gap:14px; border:0; background:transparent; color:var(--text-sub); cursor:pointer; }
.grid { display:grid; grid-template-columns:repeat(4,minmax(0,1fr)); gap:10px; }
.full { width:min(720px, calc(100vw - 36px)); max-height:82vh; overflow:auto; padding:20px; background:var(--card-bg); border-radius:16px; text-align:left; }
nav { display:flex; gap:8px; margin:10px 0 16px; }
nav button { border:0; border-radius:8px; background:var(--input-bg); color:inherit; padding:5px 12px; cursor:pointer; }
nav button[aria-pressed="true"] { background:#caedf6; color:#17323c; }
p { color:var(--text-sub); font-size:13px; }
.full > .btn-cancel { margin-top:14px; }
@media (max-width:460px) { .grid { grid-template-columns:repeat(3,minmax(0,1fr)); } .recent > :nth-child(n+4) { display:none; } }
</style>
