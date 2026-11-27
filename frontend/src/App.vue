<script setup lang="ts">
import RecycleConfirmation from "./components/download/RecycleConfirmation.vue";
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
import SettingsDrawer from "./components/settings/SettingsDrawer.vue";
import DownloadsView from "./views/DownloadsView.vue";

import type { Episode, MediaInfoResponse, SafeAccount } from "./mygo";
import { canUnlock, resolveParseTarget, useCredentials } from "./composables/useCredentials";
import { useMediaParser } from "./composables/useMediaParser";
import { useMediaPreview } from "./composables/useMediaPreview";
import { useMediaSelection } from "./composables/useMediaSelection";
import { useDownloads } from "./composables/useDownloads";
import { usePreferences } from "./composables/usePreferences";
import { getUniqueVideoStreams } from "./utils/media";

// Composables
const {
  accounts,
  logout,
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

const { isParsing, parseError, episodeError, selectedEpisodeURL, parseEpisode, mediaInfo, parse, clearMediaInfo } = useMediaParser();
const { previewMap, loadPreviews } = useMediaPreview();
const {
  selectedVideoID,
  selectedVideoCodec,
  selectedAudioID,
  selectedAudioKey,
  selectedImages,
  initSelection,
  toggleImage,
  toggleAllImages,
} = useMediaSelection();

const {
  isDownloading,
  cards,
  error: downloadError,
  doDownload,
  doCancel,
  act,
  removeMultiple,
  recycleMultiple,
  managementError,
  isRemoving,
  recycleConfirmation,
  confirmRecycle,
  refreshFileStates,
} = useDownloads();
const { preferences, setTheme, setDefaultAudioFormat, setDownloadDirectory } = usePreferences();

// State
const currentView = ref<"home" | "downloads">("home");
const drawerOpen = ref(false);
const settingsOpen = ref(false);

const logoutTarget = ref<SafeAccount>();
const logoutBusy = ref(false);
const logoutError = ref("");
const accountBusy = computed(() => isParsing.value || isPreparing.value || isDownloading.value || logoutBusy.value);
const requestLogout = (account: SafeAccount) => {
  if (accountBusy.value) return;
  logoutError.value = "";
  logoutTarget.value = account;
};
const confirmLogout = async () => {
  const account = logoutTarget.value;
  if (!account || accountBusy.value) return;
  logoutBusy.value = true;
  logoutError.value = "";
  // 即使磁盘清理失败，旧 Jar 也可能已撤销，先使资源快照失效。
  if (mediaInfo.value?.siteId === account.siteId && mediaInfo.value.usedAccountId === account.accountId) closeResourcePicker();
  closeQRModal();
  try {
    await logout(account);
    logoutTarget.value = undefined;
  } catch (error) {
    logoutError.value = error instanceof Error ? error.message : "退出失败，请重试。";
  } finally {
    logoutBusy.value = false;
  }
};

const url = ref("");
const episodeOptions = ref<Episode[]>([]);
let snapshot: { url: string; media: MediaInfoResponse } | undefined;
const isPreparing = ref(false);

const openCredentials = () => {
  showAuthManage.value = true;
  void ensureInitialized();
};

const applyResult = (url: string, media: MediaInfoResponse) => {
  snapshot = { url: media.sourceUrl || url, media };
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
    if (res) {
      episodeOptions.value = res.episodes?.length && !res.videoStreams?.length ? res.episodes : [];
      applyResult(target.url, res);
    }
  } finally {
    isPreparing.value = false;
  }
};

const closeResourcePicker = () => {
  snapshot = undefined;
  episodeOptions.value = [];
  clearMediaInfo();
};

const handleEpisode = async (episodeURL: string) => {
  if (!episodeOptions.value.some((ep) => ep.url === episodeURL)) return;
  snapshot = undefined;
  selectedVideoID.value = -1;
  selectedVideoCodec.value = "";
  selectedAudioID.value = -1;
  const res = await parseEpisode(episodeURL);
  if (res) applyResult(episodeURL, res);
};

const handleStartDownload = async (mode: string, format: string) => {
  if (isParsing.value || !snapshot || mediaInfo.value !== snapshot.media ||
    (selectedEpisodeURL.value && snapshot.url !== selectedEpisodeURL.value)) return;
  const { url, media } = snapshot;

  const params = {
    URL: url,
    Mode: mode,
    AudioFormat: format,
    CoverPreview: previewMap.value[media.coverUrl] ?? "",
    PreviewURLs:
      mode === "images"
        ? (media.images ?? [])
            .filter((img) => selectedImages.value.includes(img.index))
            .map((img) => img.url)
        : [media.coverUrl],
    AccountID: media.usedAccountId,
    OutputDir: preferences.value.downloadDirectory,
    VideoID: mode === "images" || mode === "audio" ? -1 : selectedVideoID.value,
    VideoCodec: mode === "images" || mode === "audio" ? "" : selectedVideoCodec.value,
    AudioKey: mode === "audio" || mode === "complete" ? selectedAudioKey.value : "",
    AudioID: mode === "images" || mode === "video" ? -1 : selectedAudioID.value,
    SelectedImage: media.images?.length ? selectedImages.value : [],
  };

  if ((await doDownload(params, media, previewMap.value)) && mediaInfo.value === media)
    closeResourcePicker();
};

const uniqueVideoStreams = computed(() => {
  if (!mediaInfo.value) return [];
  return getUniqueVideoStreams(mediaInfo.value.videoStreams || []);
});

const showUnlockCard = computed(() =>
  canUnlock(mediaInfo.value, isInitialized.value, getActiveAccountId("bilibili")),
);

const handleUnlock = () => {
  const original = snapshot;
  if (!original || !showUnlockCard.value || isParsing.value) return;
  void loginBilibili(async (accountId, isCurrent) => {
    const canApply = () =>
      isCurrent() && snapshot === original && mediaInfo.value === original.media;
    if (!canApply() || isParsing.value) return;
    const oldAudioId = selectedAudioID.value;
    const res = await parse(original.url, accountId, canApply, !!selectedEpisodeURL.value);
    if (res) {
      applyResult(original.url, res);
      if (res.audioStreams?.some((audio) => audio.id === oldAudioId)) {
        const audio = res.audioStreams.find(audio => audio.id === oldAudioId)!;
        selectedAudioID.value = audio.id;
        selectedAudioKey.value = `${audio.id}|${audio.codec}|${audio.kind ?? ""}`;
      }
    }
  });
};
</script>

<template>
  <RecycleConfirmation
    :action="logoutTarget ? { title: '退出账号', name: logoutTarget.name || 'Bilibili', description: '退出后将清除本地登录凭证。', confirmLabel: '退出' } : undefined"
    :busy="accountBusy" :error="logoutError"
    @cancel="logoutTarget = undefined; logoutError = ''" @confirm="confirmLogout"
  />
  <RecycleConfirmation :confirmation="recycleConfirmation" :busy="isRemoving" :error="managementError" @cancel="recycleConfirmation = undefined" @confirm="confirmRecycle" />

  <div class="app-container" :class="{ 'app-container--downloads': currentView === 'downloads' }">
    <template v-if="currentView === 'home'">
      <AppHeader @openCredentials="openCredentials" @openSettings="settingsOpen = true" />

      <main class="main-content" :class="{ 'no-downloads': cards.length === 0 }">
        <BrandWordmark />

        <LinkInput
          v-model="url"
          :isParsing="isPreparing || isParsing"
          @submit="handleParse"
          @pasteError="(msg) => (parseError = msg)"
        />

        <SupportedSites />

        <div class="error-msg" v-if="!mediaInfo && (parseError || credentialError)" role="alert">
          {{ parseError || credentialError }}
        </div>

        <DownloadGallery
          :cards="cards"
          :busy="isDownloading || isRemoving"
          :error="drawerOpen ? '' : downloadError || (recycleConfirmation ? '' : managementError)"
          @action="act"
          @cancel="doCancel"
          @openDrawer="drawerOpen = true; refreshFileStates()"
          @openView="currentView = 'downloads'; refreshFileStates()"
        />
      </main>
    </template>

    <Transition name="fade-view">
      <DownloadsView
        v-if="currentView === 'downloads'"
        :cards="cards"
        :busy="isDownloading || isRemoving"
        :error="downloadError || (recycleConfirmation ? '' : managementError)"
        @action="act"
        @cancel="doCancel"
        @back="currentView = 'home'"
        @removeSelected="removeMultiple"
      @recycleSelected="recycleMultiple"
      />
    </Transition>

    <DownloadGalleryDrawer
      :open="drawerOpen"
      :cards="cards"
      :busy="isDownloading || isRemoving"
      :error="downloadError || (recycleConfirmation ? '' : managementError)"
      @action="act"
      @cancel="doCancel"
      @close="drawerOpen = false"
      @removeSelected="removeMultiple"
      @recycleSelected="recycleMultiple"
    />

    <SettingsDrawer
      :open="settingsOpen"
      :theme="preferences.theme"
      :downloadDirectory="preferences.downloadDirectory"
      @update:downloadDirectory="setDownloadDirectory"
      :defaultAudioFormat="preferences.defaultAudioFormat"
      @close="settingsOpen = false"
      @update:theme="setTheme"
      @update:defaultAudioFormat="setDefaultAudioFormat"
    />

    <!-- Resource Selection Modal -->
    <ResourcePicker
      :episodes="episodeOptions"
      :episodeBusy="isParsing"
      :selectedEpisodeURL="selectedEpisodeURL"
      @retryEpisode="handleEpisode(selectedEpisodeURL)"
      @selectEpisode="handleEpisode"
      v-if="mediaInfo"
      :mediaInfo="mediaInfo"
      :parseError="episodeError || parseError || credentialError || downloadError"
      :downloadBusy="isDownloading"
      :previewMap="previewMap"
      :selectedVideoID="selectedVideoID"
      :selectedVideoCodec="selectedVideoCodec"
      :selectedAudioID="selectedAudioID"
      :selectedAudioKey="selectedAudioKey"
      :selectedImages="selectedImages"
      :uniqueVideoStreams="uniqueVideoStreams"
      :showUnlockCard="showUnlockCard"
      :defaultAudioFormat="preferences.defaultAudioFormat"
      @close="closeResourcePicker"
      @startDownload="handleStartDownload"
      @update:selectedVideo="
        (id, codec) => {
          const previousID = selectedVideoID;
          selectedVideoID = id;
          selectedVideoCodec = previousID === id ? codec : uniqueVideoStreams.find(group => group.stream.id === id)?.streams.find(stream => stream.codec === selectedVideoCodec)?.codec || codec;
        }
      "
      @update:selectedAudio="(id, key) => { selectedAudioID = id; selectedAudioKey = key; }"
      @toggleImage="toggleImage"
      @toggleAllImages="toggleAllImages"
      @unlock="handleUnlock"
    />

    <!-- Auth Manage Modal -->
    <CredentialModal
      v-if="showAuthManage"
      :accounts="accounts"
      :credentialError="credentialError"
      :busy="accountBusy"
      @logout="requestLogout"
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
/* Global styles 在 main.ts */
.app-container {
  overflow-y: auto;
  height: 100vh;
  position: relative;
  scrollbar-gutter: stable;
  background: var(--bg-color);
  display: flex;
  flex-direction: column;
  transition: background-color 0.18s ease;
}
/* 独立下载页由内部 .list-container 负责滚动，避免外层与列表双重滚动条。 */
.app-container--downloads {
  overflow-y: hidden;
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
  color: var(--error-text);
  text-align: center;
  margin-top: 16px;
  background: var(--error-bg);
  padding: 10px;
  border-radius: 8px;
}

.fade-view-enter-active,
.fade-view-leave-active {
  transition:
    opacity 0.22s ease,
    transform 0.22s ease;
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
