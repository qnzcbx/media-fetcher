<script setup lang="ts">
import { ref, computed } from "vue";
import AppHeader from "./components/home/AppHeader.vue";
import BrandWordmark from "./components/home/BrandWordmark.vue";
import LinkInput from "./components/home/LinkInput.vue";
import SupportedSites from "./components/home/SupportedSites.vue";
import ResourcePicker from "./components/media/ResourcePicker.vue";
import CredentialModal from "./components/credential/CredentialModal.vue";
import BilibiliQRModal from "./components/credential/BilibiliQRModal.vue";
import DownloadGallery from "./components/download/DownloadGallery.vue";
import DownloadGalleryDrawer from "./components/download/DownloadGalleryDrawer.vue";
import DownloadsView from "./views/DownloadsView.vue";

import type { MediaInfoResponse } from "./mygo";
import { canUnlock, resolveParseTarget, useCredentials } from "./composables/useCredentials";
import { useMediaParser } from "./composables/useMediaParser";
import { useMediaPreview } from "./composables/useMediaPreview";
import { useMediaSelection } from "./composables/useMediaSelection";
import { useDownloads } from "./composables/useDownloads";
import { getUniqueVideoStreams } from "./utils/media";

// Composables
const {
  accounts,
  showAuthManage,
  showBiliQRModal,
  qrUrl,
  qrStatus,
  ensureInitialized,
  isInitialized,
  credentialError,
  getActiveAccountId,
  loginBilibili,
  closeQRModal,
} = useCredentials();

const { isParsing, parseError, mediaInfo, parse, clearMediaInfo } = useMediaParser();
const { previewMap, loadPreviews } = useMediaPreview();
const {
  selectedVideoID,
  selectedVideoCodec,
  selectedAudioID,
  selectedImages,
  initSelection,
  toggleImage,
  toggleAllImages,
} = useMediaSelection();

const { isDownloading, cards, error: downloadError, doDownload, doCancel, act, removeMultiple } = useDownloads();

// State
const currentView = ref<"home" | "downloads">("home");
const drawerOpen = ref(false);

const url = ref("");
let snapshot: { url: string; media: MediaInfoResponse } | undefined;
const isPreparing = ref(false);

const openCredentials = () => {
  showAuthManage.value = true;
  void ensureInitialized();
};

const applyResult = (url: string, media: MediaInfoResponse) => {
  snapshot = { url, media };
  initSelection(media);
  void loadPreviews(media);
};

const handleParse = async () => {
  if (!url.value.trim() || isPreparing.value || isParsing.value) return;
  const input = url.value;
  let target = resolveParseTarget(input, getActiveAccountId);
  if (!target.url) {
    parseError.value = "未找到支持的链接 (Bilibili / LOFTER)";
    return;
  }

  isPreparing.value = true;
  try {
    const ready = await ensureInitialized();
    if (!ready && target.siteId === "bilibili") {
      parseError.value = credentialError.value;
      return;
    }
    target = resolveParseTarget(input, getActiveAccountId);
    if (url.value === input) url.value = target.url;
    const res = await parse(target.url, target.accountId);
    if (res) applyResult(target.url, res);
  } finally {
    isPreparing.value = false;
  }
};

const handleStartDownload = async () => {
  if (!snapshot || mediaInfo.value !== snapshot.media) return;
  const { url, media } = snapshot;

  const params = {
    URL: url,
    AccountID: media.usedAccountId,
    OutputDir: "", // 后端默认回退到系统 Downloads 目录
    VideoID: media.images?.length ? -1 : selectedVideoID.value,
    VideoCodec: media.images?.length ? "" : selectedVideoCodec.value,
    AudioID: media.images?.length ? -1 : selectedAudioID.value,
    SelectedImage: media.images?.length ? selectedImages.value : [],
  };

  if (await doDownload(params, media, previewMap.value) && mediaInfo.value === media) clearMediaInfo();
};

const uniqueVideoStreams = computed(() => {
  if (!mediaInfo.value) return [];
  return getUniqueVideoStreams(mediaInfo.value.videoStreams || []);
});

const showUnlockCard = computed(() =>
  canUnlock(mediaInfo.value, isInitialized.value, getActiveAccountId("bilibili"))
);

const handleUnlock = () => {
  const original = snapshot;
  if (!original || !showUnlockCard.value || isParsing.value) return;
  void loginBilibili(async (accountId, isCurrent) => {
    const canApply = () => isCurrent() && snapshot === original && mediaInfo.value === original.media;
    if (!canApply() || isParsing.value) return;
    const oldAudioId = selectedAudioID.value;
    const res = await parse(original.url, accountId, canApply);
    if (res) {
      applyResult(original.url, res);
      if (res.audioStreams?.some((audio) => audio.id === oldAudioId)) {
        selectedAudioID.value = oldAudioId;
      }
    }
  });
};
</script>

<template>
  <div class="app-container">
    <template v-if="currentView === 'home'">
      <AppHeader @openCredentials="openCredentials" />

      <main class="main-content" :class="{ 'no-downloads': cards.length === 0 }">
        <BrandWordmark />

        <LinkInput
          v-model="url"
          :isParsing="isPreparing || isParsing"
          @submit="handleParse"
          @pasteError="(msg) => (parseError = msg)"
        />

        <SupportedSites />

        <div class="error-msg" v-if="!mediaInfo && (parseError || credentialError)" role="alert">{{ parseError || credentialError }}</div>

        <DownloadGallery
          :cards="cards"
          :busy="isDownloading"
          :error="downloadError"
          @action="act"
          @cancel="doCancel"
          @openDrawer="drawerOpen = true"
          @openView="currentView = 'downloads'"
        />
      </main>
    </template>

    <Transition name="fade-view">
      <DownloadsView
        v-if="currentView === 'downloads'"
        :cards="cards"
        :busy="isDownloading"
        :error="downloadError"
        @action="act"
        @cancel="doCancel"
        @back="currentView = 'home'"
        @removeSelected="removeMultiple"
      />
    </Transition>

    <DownloadGalleryDrawer
      :open="drawerOpen"
      :cards="cards"
      :busy="isDownloading"
      :error="downloadError"
      @action="act"
      @cancel="doCancel"
      @close="drawerOpen = false"
      @removeSelected="removeMultiple"
    />

    <!-- Resource Selection Modal -->
    <ResourcePicker
      v-if="mediaInfo"
      :mediaInfo="mediaInfo"
      :parseError="parseError || credentialError || downloadError"
      :downloadBusy="isDownloading"
      :previewMap="previewMap"
      :selectedVideoID="selectedVideoID"
      :selectedVideoCodec="selectedVideoCodec"
      :selectedAudioID="selectedAudioID"
      :selectedImages="selectedImages"
      :uniqueVideoStreams="uniqueVideoStreams"
      :showUnlockCard="showUnlockCard"
      @close="clearMediaInfo"
      @startDownload="handleStartDownload"
      @update:selectedVideo="(id, codec) => { selectedVideoID = id; selectedVideoCodec = codec; }"
      @update:selectedAudio="(id) => (selectedAudioID = id)"
      @toggleImage="toggleImage"
      @toggleAllImages="toggleAllImages"
      @unlock="handleUnlock"
    />

    <!-- Auth Manage Modal -->
    <CredentialModal
      v-if="showAuthManage"
      :accounts="accounts"
      :credentialError="credentialError"
      @close="showAuthManage = false"
      @loginBilibili="() => loginBilibili()"
    />

    <!-- Auth QR Modal -->
    <BilibiliQRModal
      v-if="showBiliQRModal"
      :qrUrl="qrUrl"
      :qrStatus="qrStatus"
      @close="closeQRModal"
    />
  </div>
</template>

<style>
/* Global styles are injected in main.ts */
.app-container {
  overflow-y: auto;
  height: 100vh;
  position: relative;
  scrollbar-gutter: stable;
  background: var(--bg);
  display: flex;
  flex-direction: column;
}
.main-content {
  flex: 1;
  padding: 18px 20px 24px;
  max-width: 720px;
  margin: 0 auto;
  width: 100%;
  display: flex;
  flex-direction: column;
  transition: all 0.3s ease;
}

.main-content.no-downloads {
  margin-top: 4vh;
}

.error-msg {
  color: #e53e3e;
  text-align: center;
  margin-top: 16px;
  background: #fed7d7;
  padding: 10px;
  border-radius: 8px;
}

.fade-view-enter-active,
.fade-view-leave-active {
  transition: opacity 0.22s ease, transform 0.22s ease;
}

.fade-view-leave-active {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
}

.fade-view-enter-from,
.fade-view-leave-to {
  opacity: 0;
  transform: translateY(10px);
}
</style>
