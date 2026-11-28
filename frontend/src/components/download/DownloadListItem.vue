<script setup lang="ts">
import { ref, watch, computed } from "vue";
import { formatFileSize, formatDuration } from "../../utils/fileMetadata";
import { progressText } from "../../utils/downloadStatus";
import { isSelectableCard } from "../../composables/useGallerySelection";
import { useActionMenu } from "../../composables/useActionMenu";
import { Image, Play, Music, Ellipsis, Check } from "@lucide/vue";
import { downloadAPI } from "../../api/download";
import type { GalleryCard } from "../../composables/useDownloads";
import DownloadProgressOverlay from "./DownloadProgressOverlay.vue";

const props = defineProps<{
  card: GalleryCard;
  busy: boolean;
  isSelectionMode?: boolean;
  isSelected?: boolean;
}>();

const emit = defineEmits<{
  action: [card: GalleryCard, action: string];
  cancel: [];
  toggleSelect: [];
}>();

const thumbnail = ref("");
const imageFailed = ref(false);
const rowRef = ref<HTMLElement | null>(null);
const moreBtnRef = ref<HTMLElement | null>(null);
const menuRef = ref<HTMLElement | null>(null);

const { isOpen: menu, close: closeMenu, toggle: toggleMenu } = useActionMenu({
  menuRef,
  triggerRef: moreBtnRef,
  contextRootRef: rowRef,
});

watch(
  () => props.isSelectionMode,
  (selecting) => {
    if (selecting) closeMenu();
  },
);
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
  closeMenu();
  emit("action", props.card, action);
};

const handleItemClick = () => {
  if (props.isSelectionMode) {
    // 活动任务不参与移除选择。
    if (selectable.value) emit("toggleSelect");
  } else if (props.card.status === "done") {
    pick("source");
  }
};

const selectable = computed(() => isSelectableCard(props.card));

const displayTitle = computed(() => {
  if (props.card.finalPath) {
    const parts = props.card.finalPath.split(/[/\\]/);
    return parts[parts.length - 1];
  }
  return props.card.title || "未知文件";
});

const recoveredDuration = ref<number>();
let attemptedId = "";
const recoverDuration = async () => {
  const { id, status, kind, durationSeconds } = props.card;
  if (status !== "done" || kind === "image" || durationSeconds || attemptedId === id) return;
  attemptedId = id;
  recoveredDuration.value = undefined;
  try {
    const seconds = await downloadAPI.duration(id);
    if (props.card.id === id) recoveredDuration.value = seconds;
  } catch {
    /* 旧记录缺少时长时继续展示可用数据。 */
  }
};
const metadata = computed(() => {
  const size = props.card.fileSize;
  const sizeText = size ? formatFileSize(size) : "";
  const duration =
    props.card.kind !== "image"
      ? formatDuration(props.card.durationSeconds || recoveredDuration.value)
      : "";
  return [sizeText, duration].filter(Boolean).join(" · ");
});
</script>

<template>
  <div
    ref="rowRef"
    @mouseenter="recoverDuration"
    @focusin="recoverDuration"
    class="list-item"
    @click="handleItemClick"
    :class="{ selectable: isSelectionMode, selected: isSelected }"
    @contextmenu.prevent="!isSelectionMode && toggleMenu()"
    @keydown.esc="closeMenu"
  >
    <div
      v-if="isSelectionMode"
      class="checkbox"
      :class="{ selected: isSelected, disabled: !selectable }"
    >
      <Check v-if="isSelected" :size="14" color="#fff" strokeWidth="3" />
    </div>

    <div class="thumbnail-wrapper">
      <div class="cover">
        <img
          v-if="!imageFailed && (thumbnail || card.preview?.startsWith('data:image/'))"
          :src="thumbnail || card.preview"
          alt=""
          @error="imageFailed = true"
        />
        <Image v-else class="placeholder" aria-hidden="true" />
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
        <Transition name="fade"><DownloadProgressOverlay :card="card" /></Transition>
      </div>
    </div>

    <div class="info">
      <div class="title" :title="displayTitle">{{ displayTitle }}</div>
      <div class="meta">
        <span v-if="card.status === 'running' && card.progress">
          <span v-if="progressText(card.progress)">{{ progressText(card.progress) }}</span><span v-else class="loading-feedback" role="status" aria-label="处理中">◌</span>
        </span>
        <span v-else-if="card.status === 'failed'">下载失败</span>
        <span v-else-if="card.status === 'cancelled'">已取消</span>
        <span v-else-if="card.status === 'queued' || card.status === 'cancelling'">
          {{ card.status === "cancelling" ? "取消中..." : "排队中..." }}
        </span>
        <span v-else>{{ card.fileState === "missing" ? "文件不存在" : card.fileState === "unavailable" ? "文件不可用" : metadata }}</span>
      </div>
    </div>

    <div class="actions-area" v-if="!isSelectionMode">
      <button ref="moreBtnRef" class="more-btn" aria-label="操作" :aria-expanded="menu" @click.stop="toggleMenu">
        <Ellipsis :size="18" />
      </button>
      <div v-if="menu" ref="menuRef" class="dropdown-menu">
        <template v-if="card.status === 'done'">
          <button :disabled="card.fileState === 'missing' || card.fileState === 'unavailable'" @click.stop="pick('file')">打开文件</button>
          <button @click.stop="pick('folder')">打开目录</button>
          <button @click.stop="pick('copy')">复制原链接</button>
        </template>
        <template v-if="card.status === 'failed' || card.status === 'cancelled'">
          <button v-if="card.retry" :disabled="busy" @click.stop="pick('retry')">重试</button>
        </template>
        <button
          v-if="['done', 'failed', 'cancelled'].includes(card.status)"
          @click.stop="pick('recycle')"
        >
          删除
        </button>
        <button
          v-else
          :disabled="card.status === 'cancelling'"
          @click.stop="
            closeMenu();
            emit('cancel');
          "
        >
          取消下载
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.loading-feedback {display:inline-block; animation:rotate-feedback 1s linear infinite;}
@keyframes rotate-feedback {to {transform:rotate(360deg)}}
@media(prefers-reduced-motion:reduce){.loading-feedback{animation:none}}
.recycle { color:var(--danger) !important; }
.file-warning { position:absolute; bottom:5px; z-index:3; color:white; background:#202427bb; padding:3px 6px; border-radius:5px; font-size:12px; }
.list-item {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 12px 16px;
  border-bottom: 1px solid var(--border);
  position: relative;
  background: var(--card-bg);
  transition: background 0.2s;
}
.list-item:hover {
  background: var(--hover-bg);
}
.list-item.selectable {
  cursor: pointer;
}
.list-item.selected {
  background: var(--selection-soft);
}

.checkbox {
  width: 20px;
  height: 20px;
  border-radius: 50%;
  border: 2px solid var(--border);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
.checkbox.selected {
  background: var(--primary);
  border-color: var(--primary);
}
.checkbox.disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.thumbnail-wrapper {
  width: 120px;
  flex-shrink: 0;
}
.cover {
  width: 100%;
  aspect-ratio: 16/9;
  position: relative;
  border-radius: 8px;
  overflow: hidden;
  background: var(--input-bg);
  display: grid;
  place-items: center;
}
img {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.placeholder {
  width: 24px;
  height: 24px;
  color: var(--text-sub);
}
.play {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  width: 32px;
  height: 32px;
  padding: 8px;
  border-radius: 50%;
  background: rgba(19, 21, 26, 0.5);
  color: #fff;
  fill: #fff;
}
.cover :deep(.shade) {
  z-index: 2;
  gap: 4px;
  padding: 4px;
  font-size: 11px;
}

/* 列表缩略图高度较小，缩小进度遮罩的字号避免文字被裁切。 */
.cover :deep(.shade strong) {
  font-size: 16px;
}

.cover :deep(.shade progress) {
  width: 70%;
  height: 3px;
}

.info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.title {
  font-size: 14px;
  color: var(--text-main);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.meta {
  font-size: 13px;
  color: var(--text-sub);
}

.actions-area {
  position: relative;
}
.more-btn {
  background: transparent;
  border: 0;
  color: var(--text-sub);
  padding: 8px;
  cursor: pointer;
  border-radius: 6px;
}
.more-btn:hover,
.more-btn[aria-expanded="true"] {
  background: var(--border);
  color: var(--text-main);
}
.dropdown-menu {
  position: absolute;
  right: 0;
  top: 36px;
  z-index: 10;
  background: var(--card-bg);
  border: 1px solid var(--border);
  border-radius: 8px;
  box-shadow: 0 4px 12px var(--shadow-color);
  padding: 4px;
  min-width: 110px;
  display: flex;
  flex-direction: column;
}
.dropdown-menu button {
  background: none;
  border: none;
  padding: 8px 12px;
  text-align: left;
  font-size: 13px;
  color: var(--text-main);
  cursor: pointer;
  border-radius: 4px;
  white-space: nowrap;
}
.dropdown-menu button:hover {
  background: var(--hover-bg);
}

.fade-leave-active {
  transition: opacity 0.2s ease;
}
.fade-leave-to {
  opacity: 0;
}
</style>
