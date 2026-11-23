<script setup lang="ts">
import { ref, onMounted } from "vue";
import { DesktopService, events } from "./mygo";
import { Download, CheckCircle, Key, ClipboardPaste, X } from "@lucide/vue";
import QrcodeVue from "qrcode.vue";

// --- 类型定义 ---
type VideoStream = {
  id: number;
  quality_label: string;
  codec: string;
  bandwidth: number;
  need_auth: boolean;
};
type SafeAccount = {
  siteId: string;
  accountId: string;
  name: string;
  avatarUrl: string;
  status: string;
  isActive: boolean;
};

// --- 状态定义 ---
const url = ref("");
const isParsing = ref(false);
const parseError = ref("");
const mediaInfo = ref<any>(null);
const accounts = ref<SafeAccount[]>([]);

// 预览图加载
const previewMap = ref<Record<string, string>>({});

// 画质选择
const selectedVideoID = ref(-1);
const selectedVideoCodec = ref("");
const selectedAudioID = ref(-1);

// 图片选择
const selectedImages = ref<number[]>([]);

// 鉴权状态
const showAuthManage = ref(false);
const showAuthModal = ref(false);
const qrUrl = ref("");
const qrStatus = ref("");

// 下载状态
const isDownloading = ref(false);
const progressInfo = ref<any>(null);

// 加载本地账号
const fetchAccounts = async () => {
  try {
    accounts.value = (await DesktopService.getAccounts()) || [];
  } catch (e) {
    console.error(e);
  }
};

onMounted(() => {
  fetchAccounts();

  events.progress.on((info: any) => {
    progressInfo.value = info;
    if (info.stage === "done" || info.stage === "error") {
      setTimeout(() => {
        isDownloading.value = false;
      }, 5000);
    }
  });
});

const extractSupportedURL = (text: string): string => {
  const match = text.match(/(https?:\/\/[^\s]+)/);
  if (!match) return "";
  let extracted = match[0];
  // 移除尾部的中文标点或常见闭合符号
  extracted = extracted.replace(/[\]】）"'\s]+$/, "");

  if (
    extracted.includes("bilibili.com") ||
    extracted.includes("b23.tv") ||
    extracted.includes("lofter.com")
  ) {
    return extracted;
  }
  return "";
};

const pasteFromClipboard = async () => {
  try {
    const text = await navigator.clipboard.readText();
    if (text) {
      url.value = text;
    }
  } catch (err) {
    parseError.value = "无法读取剪贴板，请手动粘贴";
  }
};

const getPreview = async (imgUrl: string) => {
  if (previewMap.value[imgUrl]) return;
  try {
    const b64 = await DesktopService.fetchPreviewBase64(imgUrl, mediaInfo.value?.siteId || "");
    previewMap.value[imgUrl] = b64;
  } catch (e) {
    console.error("Failed to fetch preview for", imgUrl, e);
  }
};

const loadPreviews = async () => {
  if (!mediaInfo.value) return;
  if (mediaInfo.value.coverUrl) {
    getPreview(mediaInfo.value.coverUrl);
  }
  if (mediaInfo.value.images) {
    for (const img of mediaInfo.value.images) {
      getPreview(img.url);
    }
  }
};

const doParse = async () => {
  if (!url.value.trim() || isParsing.value) return;
  const parsedUrl = extractSupportedURL(url.value.trim());
  if (!parsedUrl) {
    parseError.value = "未找到支持的链接 (Bilibili / LOFTER)";
    return;
  }
  url.value = parsedUrl;

  isParsing.value = true;
  parseError.value = "";
  mediaInfo.value = null;
  selectedVideoID.value = -1;
  selectedVideoCodec.value = "";
  selectedAudioID.value = -1;
  selectedImages.value = [];

  try {
    let siteId = "";
    if (parsedUrl.includes("bilibili.com") || parsedUrl.includes("b23.tv")) siteId = "bilibili";
    else if (parsedUrl.includes("lofter.com")) siteId = "lofter";

    let accId = "";
    if (siteId) {
      const active = accounts.value.find((a) => a.isActive && a.siteId === siteId);
      if (active) accId = active.accountId;
    }

    const res = await DesktopService.extract(parsedUrl, accId);
    mediaInfo.value = res;

    // 默认选用第一个（也是唯一提炼出的最佳）画质
    const uniques = getUniqueVideoStreams();
    if (uniques.length > 0) {
      selectedVideoID.value = uniques[0].stream.id;
      selectedVideoCodec.value = uniques[0].stream.codec;
    }

    // 默认选中最高可用音质
    if (res.audioStreams && res.audioStreams.length > 0) {
      const bestAudio = [...res.audioStreams].sort((a, b) => b.bandwidth - a.bandwidth)[0];
      selectedAudioID.value = bestAudio.id;
    }

    // 默认选中全部图片
    if (res.images && res.images.length > 0) {
      selectedImages.value = res.images.map((img: any) => img.index);
    }

    loadPreviews();
  } catch (e: any) {
    parseError.value = e.message || "解析失败";
  } finally {
    isParsing.value = false;
  }
};

const startDownload = async () => {
  isDownloading.value = true;
  progressInfo.value = { stage: "starting", message: "准备下载..." };

  let accId = "";
  if (mediaInfo.value && mediaInfo.value.siteId) {
    const active = accounts.value.find((a) => a.isActive && a.siteId === mediaInfo.value.siteId);
    if (active) accId = active.accountId;
  }

  const currentMedia = mediaInfo.value;
  mediaInfo.value = null; // 关闭资源选择弹窗

  try {
    await DesktopService.startDownload({
      URL: url.value,
      AccountID: accId,
      OutputDir: "", // 后端默认回退到系统 Downloads 目录
      VideoID: currentMedia.images?.length ? -1 : selectedVideoID.value,
      VideoCodec: currentMedia.images?.length ? "" : selectedVideoCodec.value,
      AudioID: currentMedia.images?.length ? -1 : selectedAudioID.value,
      SelectedImage: currentMedia.images?.length ? selectedImages.value : [],
    });
  } catch (e: any) {
    console.error(e);
    progressInfo.value = { stage: "error", message: e.message };
  }
};

const cancelDownload = async () => {
  await DesktopService.cancelDownload();
  isDownloading.value = false;
  progressInfo.value = null;
};

const loginBilibili = async () => {
  showAuthManage.value = false;
  showAuthModal.value = true;
  qrStatus.value = "申请二维码中...";
  qrUrl.value = "";

  try {
    const res = await DesktopService.generateBiliQRCode();
    qrUrl.value = res.url;
    qrStatus.value = "请使用手机 Bilibili App 扫描二维码";

    // 轮询状态
    pollQRCode(res.qrcodeKey);
  } catch (e: any) {
    qrStatus.value = "错误: " + e.message;
  }
};

const pollQRCode = async (key: string) => {
  if (!showAuthModal.value) return;
  try {
    const res = await DesktopService.pollBiliQRCode(key);
    qrStatus.value = res.message;
    if (res.state === "confirmed") {
      await fetchAccounts();
      setTimeout(() => {
        showAuthModal.value = false;
      }, 1500);
    } else if (res.state === "expired" || res.state === "failed") {
      // 终止轮询
    } else {
      setTimeout(() => pollQRCode(key), 3000);
    }
  } catch (e) {
    qrStatus.value = "轮询错误";
    setTimeout(() => pollQRCode(key), 3000);
  }
};

// 精简提取唯一的视频分辨率
const getUniqueVideoStreams = () => {
  if (!mediaInfo.value || !mediaInfo.value.videoStreams) return [];
  const streams: VideoStream[] = mediaInfo.value.videoStreams;
  const unique = [];
  const seen = new Set();
  for (let i = 0; i < streams.length; i++) {
    const label = streams[i].quality_label;
    if (!seen.has(label)) {
      seen.add(label);
      unique.push({ index: i, stream: streams[i] });
    }
  }
  return unique;
};

const toggleImage = (idx: number) => {
  if (selectedImages.value.includes(idx)) {
    selectedImages.value = selectedImages.value.filter((i) => i !== idx);
  } else {
    selectedImages.value.push(idx);
  }
};

const toggleAllImages = () => {
  if (selectedImages.value.length === mediaInfo.value.images.length) {
    selectedImages.value = [];
  } else {
    selectedImages.value = mediaInfo.value.images.map((img: any) => img.index);
  }
};
</script>

<template>
  <div class="app-container">
    <header class="topbar">
      <div class="logo"></div>
      <div class="actions">
        <button @click="showAuthManage = true" class="icon-btn" title="凭证管理">
          <Key class="icon" />
        </button>
      </div>
    </header>

    <main class="main-content">
      <div class="hero">
        <h1>Media Fetcher</h1>
      </div>

      <form class="search-box" @submit.prevent="doParse">
        <input v-model="url" type="text" placeholder="你的链接" required />
        <div class="icon-btn-box">
          <button
            type="button"
            @click="pasteFromClipboard"
            class="icon-btn-small"
            title="粘贴"
            :disabled="isParsing"
          >
            <ClipboardPaste class="icon" />
          </button>
          <button type="submit" class="icon-btn-small primary" title="下载" :disabled="isParsing">
            <Download class="icon" />
          </button>
        </div>
      </form>
      <div class="support-text">Bilibili &nbsp;&nbsp; LOFTER</div>
      <div class="error-msg" v-if="parseError">{{ parseError }}</div>

      <!-- 我的下载：预留展位 -->
      <div class="my-downloads" v-if="isDownloading && progressInfo">
        <h3>我的下载 ></h3>
        <div class="download-task">
          <div class="task-header">
            <h4>下载任务</h4>
            <span class="status-badge" :class="progressInfo.stage">{{ progressInfo.stage }}</span>
          </div>
          <div class="task-msg">{{ progressInfo.message }}</div>
          <div
            class="progress-bar-container"
            v-if="progressInfo.stage !== 'done' && progressInfo.stage !== 'error'"
          >
            <div
              class="progress-bar"
              :style="{
                width:
                  progressInfo.total > 0
                    ? (progressInfo.downloaded / progressInfo.total) * 100 + '%'
                    : '100%',
              }"
              :class="{ indeterminate: progressInfo.total <= 0 }"
            ></div>
          </div>
          <div class="task-actions">
            <button
              v-if="progressInfo.stage !== 'done' && progressInfo.stage !== 'error'"
              @click="cancelDownload"
              class="btn-cancel"
            >
              取消
            </button>
          </div>
        </div>
      </div>
    </main>

    <!-- Resource Selection Modal -->
    <div class="modal-overlay" v-if="mediaInfo">
      <div class="resource-modal">
        <div class="rmodal-header">
          <h3>选择资源</h3>
          <button class="icon-btn" @click="mediaInfo = null"><X class="icon" /></button>
        </div>
        <div class="rmodal-body">
          <!-- Videos -->
          <div v-if="mediaInfo.videoStreams && mediaInfo.videoStreams.length > 0">
            <div class="video-grid">
              <div
                v-for="item in getUniqueVideoStreams()"
                :key="item.index"
                class="video-card"
                :class="{
                  active:
                    selectedVideoID === item.stream.id && selectedVideoCodec === item.stream.codec,
                }"
                @click="
                  selectedVideoID = item.stream.id;
                  selectedVideoCodec = item.stream.codec;
                "
              >
                <div class="video-cover">
                  <img
                    v-if="previewMap[mediaInfo.coverUrl]"
                    :src="previewMap[mediaInfo.coverUrl]"
                  />
                  <div v-else class="preview-placeholder">封面加载中...</div>
                  <div
                    class="check"
                    v-if="
                      selectedVideoID === item.stream.id && selectedVideoCodec === item.stream.codec
                    "
                  >
                    <CheckCircle :size="20" />
                  </div>
                </div>
                <div class="video-quality">{{ item.stream.quality_label }}</div>
              </div>
            </div>

            <div class="audio-select-container">
              <span>音质</span>
              <select class="audio-select" v-model="selectedAudioID">
                <option v-for="as in mediaInfo.audioStreams" :key="as.id" :value="as.id">
                  {{ as.quality }}
                </option>
              </select>
            </div>
          </div>

          <!-- Images -->
          <div
            v-if="
              mediaInfo.images &&
              mediaInfo.images.length > 0 &&
              (!mediaInfo.videoStreams || mediaInfo.videoStreams.length === 0)
            "
          >
            <div class="image-header">
              <label class="custom-checkbox" @click="toggleAllImages">
                <input
                  type="checkbox"
                  :checked="selectedImages.length === mediaInfo.images.length"
                />
                <span>全部 ({{ selectedImages.length }}/{{ mediaInfo.images.length }})</span>
              </label>
            </div>
            <div class="video-grid">
              <div
                v-for="img in mediaInfo.images"
                :key="img.index"
                class="video-card"
                :class="{ active: selectedImages.includes(img.index) }"
                @click="toggleImage(img.index)"
              >
                <div class="video-cover image-cover">
                  <img v-if="previewMap[img.url]" :src="previewMap[img.url]" />
                  <div v-else class="preview-placeholder">加载中...</div>
                  <div class="check" v-if="selectedImages.includes(img.index)">
                    <CheckCircle :size="20" />
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div class="rmodal-footer">
          <button
            class="btn-primary"
            style="width: 100%; justify-content: center"
            @click="startDownload"
            :disabled="selectedImages.length === 0 && !mediaInfo.videoStreams?.length"
          >
            下载
          </button>
        </div>
      </div>
    </div>

    <!-- Auth Manage Modal -->
    <div class="modal-overlay" v-if="showAuthManage">
      <div class="modal auth-modal">
        <h3>凭证管理</h3>

        <div class="auth-card">
          <h4>Bilibili</h4>
          <div
            v-if="accounts.find((a) => a.isActive && a.siteId === 'bilibili')"
            class="auth-status active"
          >
            {{ accounts.find((a) => a.isActive && a.siteId === "bilibili")?.name }} (已登录)
          </div>
          <div v-else class="auth-action">
            <button
              class="btn-primary"
              style="width: 100%; justify-content: center"
              @click="loginBilibili"
            >
              登录 Bilibili
            </button>
          </div>
        </div>

        <div class="auth-card">
          <h4>LOFTER</h4>
          <div class="auth-status anonymous">匿名模式 (当前未配置凭证)</div>
        </div>

        <button
          class="btn-cancel"
          style="margin-top: 24px; width: 100%"
          @click="showAuthManage = false"
        >
          关闭
        </button>
      </div>
    </div>

    <!-- Auth QR Modal -->
    <div class="modal-overlay" v-if="showAuthModal">
      <div class="modal auth-modal">
        <h3>登录 Bilibili</h3>
        <div class="qr-container">
          <qrcode-vue v-if="qrUrl" :value="qrUrl" :size="200" level="M" render-as="svg" />
          <div v-else class="qr-placeholder">加载中...</div>
        </div>
        <p class="qr-status">{{ qrStatus }}</p>
        <button class="btn-cancel" @click="showAuthModal = false">关闭</button>
      </div>
    </div>
  </div>
</template>

<style>
@font-face {
  font-family: "Parisienne";
  src: url("./assets/fonts/Parisienne.ttf") format("truetype");
  font-weight: normal;
  font-style: normal;
}
@font-face {
  font-family: "Pinyon Script";
  src: url("./assets/fonts/PinyonScript.ttf") format("truetype");
  font-weight: normal;
  font-style: normal;
}

:root {
  --bg-color: #fafafa;
  --card-bg: #ffffff;
  --input-bg: #f1f3f4;
  --primary: #8ed7eb;
  --primary-hover: #72c7df;
  --text-main: #202427;
  --text-sub: #858b90;
  --border: #e0e4e8;
}

body {
  margin: 0;
  font-family:
    "Microsoft YaHei",
    -apple-system,
    BlinkMacSystemFont,
    sans-serif;
  background-color: var(--bg-color);
  color: var(--text-main);
}

.app-container {
  display: flex;
  flex-direction: column;
  height: 100vh;
  overflow: hidden;
}

.topbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 24px;
  z-index: 10;
  -webkit-app-region: drag;
}

.logo {
  font-weight: 600;
  font-size: 18px;
  display: flex;
  align-items: center;
  gap: 8px;
}

.actions {
  -webkit-app-region: no-drag;
}

.icon-btn {
  background: none;
  border: none;
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--text-main);
  cursor: pointer;
  padding: 6px 12px;
  border-radius: 6px;
  transition: background 0.2s;
}

.icon-btn:hover {
  background: var(--input-bg);
}

.icon-btn-box {
  display: flex;
  gap: 6px;
}

.icon-btn-small {
  background: transparent;
  border: none;
  color: var(--text-sub);
  padding: 8px;
  cursor: pointer;
  border-radius: 6px;
  display: flex;
  align-items: center;
  transition: all 0.2s;
}

.icon-btn-small:hover:not(:disabled) {
  background: var(--input-bg);
  color: var(--text-main);
}

.icon-btn-small.primary {
  color: var(--primary-hover);
}

.icon-btn-small.primary:hover:not(:disabled) {
  background: var(--primary);
  color: #000;
}

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

.hero {
  text-align: center;
  margin-bottom: 40px;
}

.hero h1 {
  font-size: 64px;
  font-family: "Parisienne", "Georgia", "Times New Roman", cursive;
  margin: 0 0 12px 0;
  font-weight: normal;
  color: var(--text-main);
}

.search-box {
  display: flex;
  background: var(--input-bg);
  border-radius: 12px;
  padding: 8px 16px;
  align-items: center;
  border: 1px solid transparent;
  transition:
    border-color 0.3s,
    background 0.3s;
}

.search-box:focus-within {
  background: var(--card-bg);
  border-color: var(--primary);
  box-shadow: 0 4px 16px rgba(142, 215, 235, 0.1);
}

.search-box input {
  flex: 1;
  border: none;
  outline: none;
  font-size: 16px;
  background: transparent;
  padding: 8px 0;
}

.support-text {
  text-align: center;
  color: var(--text-sub);
  font-size: 14px;
  margin-top: 16px;
  letter-spacing: 1px;
}

.error-msg {
  color: #e53e3e;
  text-align: center;
  margin-top: 16px;
  background: #fed7d7;
  padding: 10px;
  border-radius: 8px;
}

.my-downloads {
  margin-top: 64px;
}

.my-downloads h3 {
  font-size: 16px;
  font-weight: normal;
  color: var(--text-sub);
  margin-bottom: 16px;
}

.btn-primary {
  background: var(--primary);
  color: #000;
  border: none;
  padding: 12px 24px;
  border-radius: 8px;
  font-weight: 600;
  font-size: 15px;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 8px;
  transition: background 0.2s;
}

.btn-primary:hover:not(:disabled) {
  background: var(--primary-hover);
}

.btn-primary:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn-cancel {
  background: #fff;
  border: 1px solid var(--border);
  padding: 8px 16px;
  border-radius: 6px;
  cursor: pointer;
}

.btn-cancel:hover {
  background: var(--input-bg);
}

.download-task {
  background: var(--card-bg);
  border-radius: 12px;
  padding: 20px;
  border: 1px solid var(--border);
}

.task-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.task-header h4 {
  margin: 0;
  font-size: 16px;
}

.status-badge {
  font-size: 12px;
  padding: 4px 8px;
  border-radius: 12px;
  background: var(--input-bg);
  text-transform: capitalize;
}

.status-badge.done {
  background: #c6f6d5;
  color: #22543d;
}
.status-badge.error {
  background: #fed7d7;
  color: #822727;
}
.status-badge.muxing {
  background: #feebc8;
  color: #7b341e;
}

.task-msg {
  color: var(--text-sub);
  font-size: 14px;
  margin-bottom: 16px;
}

.progress-bar-container {
  height: 6px;
  background: var(--input-bg);
  border-radius: 3px;
  overflow: hidden;
  margin-bottom: 16px;
}

.progress-bar {
  height: 100%;
  background: var(--primary);
  transition: width 0.3s ease;
}

.progress-bar.indeterminate {
  width: 30%;
  animation: indeterminate 1.5s infinite linear;
}

@keyframes indeterminate {
  0% {
    transform: translateX(-100%);
  }
  100% {
    transform: translateX(330%);
  }
}

.task-actions {
  display: flex;
  justify-content: flex-end;
}

/* Modals */
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.4);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
}

.modal.auth-modal {
  background: var(--card-bg);
  padding: 32px;
  border-radius: 16px;
  width: 320px;
  text-align: center;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.1);
}

.modal h3 {
  margin: 0 0 24px 0;
}

.qr-container {
  width: 200px;
  height: 200px;
  margin: 0 auto 16px;
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 8px;
}

.qr-container img {
  width: 100%;
  height: 100%;
}

.qr-placeholder {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-sub);
  background: var(--input-bg);
}

.qr-status {
  color: var(--text-sub);
  font-size: 14px;
  margin-bottom: 24px;
}

.auth-card {
  text-align: left;
  background: var(--input-bg);
  padding: 16px;
  border-radius: 8px;
  margin-bottom: 16px;
}
.auth-card h4 {
  margin: 0 0 12px 0;
  font-size: 15px;
}
.auth-status {
  font-size: 14px;
  color: var(--text-sub);
}
.auth-status.active {
  color: #2b6cb0;
  font-weight: 500;
}

/* Resource Selection Modal */
.resource-modal {
  background: var(--card-bg);
  border-radius: 16px;
  width: 680px;
  max-width: 90vw;
  max-height: 85vh;
  display: flex;
  flex-direction: column;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.1);
}

.rmodal-header {
  padding: 20px 24px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  border-bottom: 1px solid var(--border);
}

.rmodal-header h3 {
  margin: 0;
  font-size: 18px;
}

.rmodal-body {
  padding: 24px;
  overflow-y: auto;
  flex: 1;
}

.rmodal-footer {
  padding: 20px 24px;
  border-top: 1px solid var(--border);
}

.video-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 16px;
  margin-bottom: 24px;
}

.video-card {
  background: var(--card-bg);
  border-radius: 8px;
  border: 2px solid transparent;
  cursor: pointer;
  transition: all 0.2s;
  overflow: hidden;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.05);
}

.video-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}

.video-card.active {
  border-color: var(--primary);
}

.video-cover {
  width: 100%;
  aspect-ratio: 16/9;
  background: var(--input-bg);
  position: relative;
}

.video-cover.image-cover {
  aspect-ratio: 1;
}

.video-cover img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.preview-placeholder {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-sub);
  font-size: 12px;
}

.video-card .check {
  position: absolute;
  top: 8px;
  right: 8px;
  color: var(--primary);
  background: #fff;
  border-radius: 50%;
  display: flex;
}

.video-quality {
  padding: 12px;
  text-align: center;
  font-weight: 500;
}

.audio-select-container {
  display: flex;
  align-items: center;
  gap: 12px;
  background: var(--input-bg);
  padding: 12px 16px;
  border-radius: 8px;
}

.audio-select {
  border: none;
  background: transparent;
  font-size: 14px;
  outline: none;
  flex: 1;
  color: var(--text-main);
  cursor: pointer;
}

.image-header {
  margin-bottom: 16px;
}

.custom-checkbox {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  user-select: none;
}
</style>
