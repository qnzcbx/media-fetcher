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
import { getSiteIdFromURL, extractSupportedURL } from "./utils/url";

// Composables
const {
  accounts,
  showAuthManage,
  showBiliQRModal,
  qrUrl,
  qrStatus,
  ensureInitialized,
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

const { isDownloading, progressInfo, doDownload, doCancel } = useDownloads();

// State
const url = ref("");
let normalizedUrl = "";
let usedAccountId = "";

// Event Handlers
const handleParse = async () => {
  if (!url.value.trim() || isParsing.value) return;

  await ensureInitialized(); // 确保账号已加载

  const parsedUrl = extractSupportedURL(url.value.trim());
  if (!parsedUrl) {
    parseError.value = "未找到支持的链接 (Bilibili / LOFTER)";
    return;
  }
  url.value = parsedUrl;

  const siteId = getSiteIdFromURL(parsedUrl);
  const accId = siteId ? getActiveAccountId(siteId) : "";

  const res = await parse(parsedUrl, accId);
  if (res) {
    normalizedUrl = parsedUrl; // 记录本次成功解析的 URL
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

const showUnlockCard = computed(() => {
  if (!mediaInfo.value) return false;
  const siteId = getSiteIdFromURL(normalizedUrl);
  return siteId === "bilibili" && !usedAccountId && !getActiveAccountId("bilibili");
});

const handleUnlock = () => {
  loginBilibili(async () => {
    const siteId = getSiteIdFromURL(normalizedUrl);
    const accId = siteId ? getActiveAccountId(siteId) : "";
    if (!accId) return;

    // Preserve old selection
    const oldAudioId = selectedAudioID.value;

    const res = await parse(normalizedUrl, accId);
    if (res) {
      usedAccountId = accId;
      initSelection(res);
      loadPreviews(res);

      // Try to restore old audio choice
      if (res.audioStreams?.some((a: any) => a.id === oldAudioId)) {
        selectedAudioID.value = oldAudioId;
      }
    }
  });
};
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
