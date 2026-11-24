<script setup lang="ts">
import { ref, computed } from "vue";
import AppHeader from "./components/home/AppHeader.vue";
import BrandWordmark from "./components/home/BrandWordmark.vue";
import LinkInput from "./components/home/LinkInput.vue";
import SupportedSites from "./components/home/SupportedSites.vue";
import ResourcePicker from "./components/media/ResourcePicker.vue";
import CredentialModal from "./components/credential/CredentialModal.vue";
import BilibiliQRModal from "./components/credential/BilibiliQRModal.vue";
import DownloadStatus from "./components/download/DownloadStatus.vue";

import { useCredentials } from "./composables/useCredentials";
import { useMediaParser } from "./composables/useMediaParser";
import { useMediaPreview } from "./composables/useMediaPreview";
import { useMediaSelection } from "./composables/useMediaSelection";
import { useDownloads } from "./composables/useDownloads";
import { getUniqueVideoStreams } from "./utils/media";
import { getSiteIdFromURL } from "./utils/url";

// Composables
const {
  accounts,
  showAuthManage,
  showBiliQRModal,
  qrUrl,
  qrStatus,
  getActiveAccountId,
  loginBilibili,
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

const { isDownloading, progressInfo, doDownload, doCancel } = useDownloads();

// State
const url = ref("");
let normalizedUrl = "";
let usedAccountId = "";

// Event Handlers
const handleParse = async () => {
  if (!url.value.trim() || isParsing.value) return;

  const siteId = getSiteIdFromURL(url.value.trim());
  const accId = siteId ? getActiveAccountId(siteId) : "";

  const res = await parse(url.value.trim(), accId);
  if (res) {
    normalizedUrl = url.value.trim(); // 记录本次成功解析的 URL
    usedAccountId = accId;
    initSelection(res);
    loadPreviews(res);
  }
};

const handleStartDownload = async () => {
  if (!mediaInfo.value) return;

  const params = {
    URL: normalizedUrl, // 使用上次解析成功的规范化 URL
    AccountID: usedAccountId, // 使用当时的 account
    OutputDir: "", // 后端默认回退到系统 Downloads 目录
    VideoID: mediaInfo.value.images?.length ? -1 : selectedVideoID.value,
    VideoCodec: mediaInfo.value.images?.length ? "" : selectedVideoCodec.value,
    AudioID: mediaInfo.value.images?.length ? -1 : selectedAudioID.value,
    SelectedImage: mediaInfo.value.images?.length ? selectedImages.value : [],
  };

  clearMediaInfo(); // 关闭弹窗
  await doDownload(params);
};

const uniqueVideoStreams = computed(() => {
  if (!mediaInfo.value) return [];
  return getUniqueVideoStreams(mediaInfo.value.videoStreams || []);
});
</script>

<template>
  <div class="app-container">
    <AppHeader @openCredentials="showAuthManage = true" />

    <main class="main-content">
      <BrandWordmark />

      <LinkInput
        v-model="url"
        :isParsing="isParsing"
        @submit="handleParse"
        @pasteError="(msg) => (parseError = msg)"
      />

      <SupportedSites />

      <div class="error-msg" v-if="parseError">{{ parseError }}</div>

      <DownloadStatus
        v-if="isDownloading && progressInfo"
        :progressInfo="progressInfo"
        @cancel="doCancel"
      />
    </main>

    <!-- Resource Selection Modal -->
    <ResourcePicker
      v-if="mediaInfo"
      :mediaInfo="mediaInfo"
      :previewMap="previewMap"
      :selectedVideoID="selectedVideoID"
      :selectedVideoCodec="selectedVideoCodec"
      :selectedAudioID="selectedAudioID"
      :selectedImages="selectedImages"
      :uniqueVideoStreams="uniqueVideoStreams"
      @close="clearMediaInfo"
      @startDownload="handleStartDownload"
      @update:selectedVideo="(id, codec) => { selectedVideoID = id; selectedVideoCodec = codec; }"
      @update:selectedAudio="(id) => (selectedAudioID = id)"
      @toggleImage="toggleImage"
      @toggleAllImages="toggleAllImages"
    />

    <!-- Auth Manage Modal -->
    <CredentialModal
      v-if="showAuthManage"
      :accounts="accounts"
      @close="showAuthManage = false"
      @loginBilibili="loginBilibili"
    />

    <!-- Auth QR Modal -->
    <BilibiliQRModal
      v-if="showBiliQRModal"
      :qrUrl="qrUrl"
      :qrStatus="qrStatus"
      @close="showBiliQRModal = false"
    />
  </div>
</template>

<style>
/* Global styles are injected in main.ts */
.main-content {
  flex: 1;
  padding: 80px 20px 40px;
  max-width: 720px;
  margin: 0 auto;
  width: 100%;
  display: flex;
  flex-direction: column;
  transition: all 0.3s ease;
}

.error-msg {
  color: #e53e3e;
  text-align: center;
  margin-top: 16px;
  background: #fed7d7;
  padding: 10px;
  border-radius: 8px;
}
</style>
