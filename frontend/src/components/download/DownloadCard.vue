<script setup lang="ts">
import { ref, watch } from "vue";
import { Image, Play, Music, Ellipsis } from "@lucide/vue";
import { downloadAPI } from "../../api/download";
import type { GalleryCard } from "../../composables/useDownloads";
import DownloadProgressOverlay from "./DownloadProgressOverlay.vue";

const props = defineProps<{ card: GalleryCard; busy: boolean; hideMore?: boolean }>();
const emit = defineEmits<{ action: [card: GalleryCard, action: string]; cancel: [] }>();
const thumbnail = ref("");
const menu = ref(false);
const imageFailed = ref(false);
watch(
  () => [thumbnail.value, props.card.preview],
  () => {
    imageFailed.value = false;
  },
);

watch(
  () => [props.card.id, props.card.thumbnail],
  async (_, __, onCleanup) => {
    let current = true;
    onCleanup(() => {
      current = false;
    });
    thumbnail.value = "";
    if (!props.card.thumbnail) return;
    try {
      const data = await downloadAPI.thumbnail(props.card.id);
      if (current) thumbnail.value = data;
    } catch {
      /* 无可用缩略图时显示本地占位。 */
    }
  },
  { immediate: true },
);

const pick = (action: string) => {
  menu.value = false;
  emit("action", props.card, action);
};
</script>

<template>
  <article class="tile" @contextmenu.prevent="menu = !menu" @keydown.esc="menu = false">
    <button
      class="cover"
      :title="card.title"
      :aria-label="card.title || '下载资源'"
      @click="card.status === 'done' ? pick('source') : (menu = !menu)"
    >
      <img
        v-if="!imageFailed && (thumbnail || card.preview?.startsWith('data:image/'))"
        :src="thumbnail || card.preview"
        alt=""
        @error="imageFailed = true"
      />
      <Image class="placeholder" aria-hidden="true" />
      <Music
        v-if="card.kind === 'audio' && card.status === 'done'"
        class="play"
        aria-hidden="true"
      />
      <Play
        v-if="card.kind === 'video' && card.status === 'done'"
        class="play"
        aria-hidden="true"
      />
      <span v-if="card.fileState === 'missing' || card.fileState === 'unavailable'" class="file-warning">{{ card.fileState === "missing" ? "文件不存在" : "文件不可用" }}</span>
      <Transition name="fade"><DownloadProgressOverlay :card="card" /></Transition>
    </button>
    <button v-if="!hideMore" class="more" aria-label="下载操作" :aria-expanded="menu" @click="menu = !menu">
      <Ellipsis :size="16" />
    </button>
    <div v-if="menu" class="actions">
      <template v-if="card.status === 'done'">
        <button v-if="card.canRecycle" class="recycle" @click.stop="pick('recycle')">删除文件</button>
        <button :disabled="card.fileState === 'missing' || card.fileState === 'unavailable'" @click="pick('file')">打开文件</button>
        <button @click="pick('folder')">打开目录</button>
        <button @click="pick('copy')">复制原链接</button>
      </template>
      <template v-if="card.status === 'failed' || card.status === 'cancelled'">
        <button v-if="card.retry" :disabled="busy" @click="pick('retry')">重试</button>
      </template>
      <button v-if="['done', 'failed', 'cancelled'].includes(card.status)" @click="pick('remove')">
        移除记录
      </button>
      <button
        v-else
        :disabled="card.status === 'cancelling'"
        @click="
          menu = false;
          emit('cancel');
        "
      >
        取消下载
      </button>
    </div>
  </article>
</template>

<style scoped>
.recycle { color:var(--danger) !important; }
.file-warning { position:absolute; bottom:5px; z-index:3; color:white; background:#202427bb; padding:3px 6px; border-radius:5px; font-size:12px; }
.tile {
  position: relative;
  min-width: 0;
}
button {
  cursor: pointer;
  font: inherit;
}
.cover {
  display: grid;
  place-items: center;
  position: relative;
  width: 100%;
  aspect-ratio: 1;
  overflow: hidden;
  padding: 0;
  border: 1px solid var(--border, #dce2e5);
  border-radius: 14px;
  background: var(--input-bg);
  color: var(--text-sub);
}
img {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  object-fit: cover;
  z-index: 1;
}
.placeholder {
  position: absolute;
  width: 23px;
  height: 23px;
}
.play {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  z-index: 2;
  width: 46px;
  height: 46px;
  padding: 12px;
  border-radius: 50%;
  background: #13151a88;
  color: #fff;
  fill: #fff;
}
.cover :deep(.shade) {
  z-index: 2;
}
.more {
  position: absolute;
  right: 5px;
  top: 5px;
  z-index: 3;
  display: grid;
  place-items: center;
  border: 0;
  border-radius: 6px;
  padding: 3px;
  background: #202427aa;
  color: white;
  opacity: 0;
}
.tile:hover .more,
.more:focus-visible,
.more[aria-expanded="true"] {
  opacity: 1;
}
.actions {
  position: absolute;
  z-index: 5;
  right: 0;
  top: 30px;
  display: grid;
  min-width: 106px;
  background: var(--card-bg);
  color: var(--text-main);
  border: 1px solid var(--border, #dce2e5);
  border-radius: 8px;
  padding: 4px;
  box-shadow: 0 4px 12px var(--shadow-color);
}
.actions button {
  border: 0;
  background: none;
  color: inherit;
  text-align: left;
  padding: 6px 8px;
  font-size: 12px;
  white-space: nowrap;
  border-radius: 4px;
}
.actions button:hover {
  background: var(--input-bg);
}
.fade-leave-active {
  transition: opacity 0.22s ease;
}
.fade-leave-to {
  opacity: 0;
}
</style>
