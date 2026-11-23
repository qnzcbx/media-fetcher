<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { DesktopService, events } from './mygo'
import { Search, Image, User, Download, CheckCircle } from '@lucide/vue'
import QrcodeVue from 'qrcode.vue'

// --- 类型定义 ---
type VideoStream = { id: number, quality_label: string, codec: string, bandwidth: number, need_auth: boolean }
type SafeAccount = { siteId: string, accountId: string, name: string, avatarUrl: string, status: string, isActive: boolean }

// --- 状态定义 ---
const url = ref('')
const isParsing = ref(false)
const parseError = ref('')
const mediaInfo = ref<any>(null)
const accounts = ref<SafeAccount[]>([])

// 画质选择
const selectedVideoID = ref(-1)
const selectedVideoCodec = ref('')
const selectedAudioID = ref(-1)

// 鉴权状态
const showAuthModal = ref(false)
const qrUrl = ref('')
const qrStatus = ref('')

// 下载状态
const isDownloading = ref(false)
const progressInfo = ref<any>(null)

// 加载本地账号
const fetchAccounts = async () => {
  try {
    accounts.value = await DesktopService.getAccounts() || []
  } catch (e) {
    console.error(e)
  }
}

onMounted(() => {
  fetchAccounts()

  events.progress.on((info: any) => {
    progressInfo.value = info
    if (info.stage === 'done' || info.stage === 'error') {
      setTimeout(() => {
        isDownloading.value = false
      }, 5000)
    }
  })
})

const doParse = async () => {
  if (!url.value.trim() || isParsing.value) return
  isParsing.value = true
  parseError.value = ''
  mediaInfo.value = null
  selectedVideoID.value = -1
  selectedVideoCodec.value = ''
  selectedAudioID.value = -1

  try {
    let accId = ''
    if (accounts.value.length > 0) {
      const active = accounts.value.find(a => a.isActive)
      if (active) accId = active.accountId
    }

    const res = await DesktopService.extract(url.value, accId)
    mediaInfo.value = res

    // 默认选用第一个（也是唯一提炼出的最佳）画质
    const uniques = getUniqueVideoStreams()
    if (uniques.length > 0) {
       selectedVideoID.value = uniques[0].stream.id
       selectedVideoCodec.value = uniques[0].stream.codec
    }

    // 默认选中最高可用音质
    if (res.audioStreams && res.audioStreams.length > 0) {
       // 通过降低排序或者寻最高码率，假设这里最高带宽在头部
       const bestAudio = [...res.audioStreams].sort((a, b) => b.bandwidth - a.bandwidth)[0]
       selectedAudioID.value = bestAudio.id
    }
  } catch (e: any) {
    parseError.value = e.message || '解析失败'
  } finally {
    isParsing.value = false
  }
}

const startDownload = async () => {
  isDownloading.value = true
  progressInfo.value = { stage: 'starting', message: '准备下载...' }

  let accId = ''
  const active = accounts.value.find(a => a.isActive)
  if (active) accId = active.accountId

  try {
    await DesktopService.startDownload({
      URL: url.value,
      AccountID: accId,
      OutputDir: '', // 后端默认回退到系统 Downloads 目录
      VideoID: mediaInfo.value.images?.length ? -1 : selectedVideoID.value,
      VideoCodec: mediaInfo.value.images?.length ? '' : selectedVideoCodec.value,
      AudioID: mediaInfo.value.images?.length ? -1 : selectedAudioID.value,
    })
  } catch (e: any) {
    console.error(e)
    progressInfo.value = { stage: 'error', message: e.message }
  }
}

const cancelDownload = async () => {
  await DesktopService.cancelDownload()
  isDownloading.value = false
  progressInfo.value = null
}

const loginBilibili = async () => {
  showAuthModal.value = true
  qrStatus.value = '申请二维码中...'
  qrUrl.value = ''

  try {
    const res = await DesktopService.generateBiliQRCode()
    qrUrl.value = res.url
    qrStatus.value = '请使用手机 Bilibili App 扫描二维码'

    // 轮询状态
    pollQRCode(res.qrcodeKey)
  } catch (e: any) {
    qrStatus.value = '错误: ' + e.message
  }
}

const pollQRCode = async (key: string) => {
  if (!showAuthModal.value) return
  try {
    const res = await DesktopService.pollBiliQRCode(key)
    qrStatus.value = res.message
    if (res.state === 'confirmed') {
      await fetchAccounts()
      setTimeout(() => { showAuthModal.value = false }, 1500)
    } else if (res.state === 'expired' || res.state === 'failed') {
      // 终止轮询
    } else {
      setTimeout(() => pollQRCode(key), 3000)
    }
  } catch (e) {
    qrStatus.value = '轮询错误'
    setTimeout(() => pollQRCode(key), 3000)
  }
}

// 精简提取唯一的视频分辨率
const getUniqueVideoStreams = () => {
  if (!mediaInfo.value || !mediaInfo.value.videoStreams) return []
  const streams: VideoStream[] = mediaInfo.value.videoStreams
  const unique = []
  const seen = new Set()
  for (let i = 0; i < streams.length; i++) {
    // 简洁模式下：对同一分辨率只展示质量最好的编码（AVC）
    const label = streams[i].quality_label
    if (!seen.has(label)) {
      seen.add(label)
      unique.push({ index: i, stream: streams[i] })
    }
  }
  return unique
}
</script>

<template>
  <div class="app-container">
    <header class="topbar">
      <div class="logo">
        <Download class="icon" /> Media Fetcher
      </div>
      <div class="actions">
        <button @click="loginBilibili" class="icon-btn">
          <User class="icon" />
          <span v-if="accounts.length > 0">{{ accounts.find(a => a.isActive)?.name || '已登录' }}</span>
          <span v-else>登录账号</span>
        </button>
      </div>
    </header>

    <main class="main-content" :class="{ 'has-result': mediaInfo }">
      <div class="hero" v-if="!mediaInfo">
        <h1>下载，从一个链接开始。</h1>
        <p>粘贴视频或图片链接，选择你想要的清晰度。</p>
      </div>

      <form class="search-box" @submit.prevent="doParse">
        <Search class="search-icon" />
        <input
          v-model="url"
          type="url"
          placeholder="https://www.bilibili.com/video/..."
          required
        />
        <button type="submit" :disabled="isParsing">
          {{ isParsing ? '解析中...' : '解析' }}
        </button>
      </form>
      <div class="support-text" v-if="!mediaInfo">Bilibili / LOFTER</div>
      <div class="error-msg" v-if="parseError">{{ parseError }}</div>

      <!-- Result View -->
      <transition name="fade">
        <div class="result-view" v-if="mediaInfo">
          <div class="media-card">
            <div class="media-info">
              <h2>{{ mediaInfo.title }}</h2>
              <div class="meta">
                <span class="tag">{{ mediaInfo.siteId }}</span>
                <span v-if="mediaInfo.author">{{ mediaInfo.author }}</span>
              </div>
            </div>
          </div>

          <!-- Videos -->
          <div v-if="mediaInfo.videoStreams && mediaInfo.videoStreams.length > 0" class="options-section">
            <h3>选择画质</h3>
            <div class="quality-grid">
              <div
                v-for="item in getUniqueVideoStreams()"
                :key="item.index"
                class="quality-card"
                :class="{ active: selectedVideoID === item.stream.id && selectedVideoCodec === item.stream.codec }"
                @click="selectedVideoID = item.stream.id; selectedVideoCodec = item.stream.codec"
              >
                <div class="q-title">{{ item.stream.quality_label }}</div>
                <div class="q-sub">{{ item.stream.codec }}</div>
                <div class="check" v-if="selectedVideoID === item.stream.id && selectedVideoCodec === item.stream.codec">
                  <CheckCircle :size="16"/>
                </div>
              </div>
            </div>
          </div>

          <!-- Images -->
          <div v-if="mediaInfo.images && mediaInfo.images.length > 0 && (!mediaInfo.videoStreams || mediaInfo.videoStreams.length===0)" class="options-section">
            <h3>图片列表 (共 {{ mediaInfo.images.length }} 张)</h3>
            <div class="image-grid">
              <div v-for="img in mediaInfo.images.slice(0,6)" :key="img.index" class="img-preview">
                <Image class="img-icon" />
                <span>{{ img.index }}</span>
              </div>
              <div v-if="mediaInfo.images.length > 6" class="img-preview more">
                +{{ mediaInfo.images.length - 6 }}
              </div>
            </div>
          </div>

          <!-- Download Action Bar -->
          <div class="download-bar" v-if="!isDownloading">
            <div class="dir-info">保存至系统默认下载目录</div>
            <button class="btn-primary" @click="startDownload">
              <Download class="icon" />
              {{ mediaInfo.images?.length && (!mediaInfo.videoStreams?.length) ? '下载全部图片' : '下载视频' }}
            </button>
          </div>

          <!-- Downloading State -->
          <div class="download-task" v-if="isDownloading && progressInfo">
            <div class="task-header">
              <h4>{{ mediaInfo.title }}</h4>
              <span class="status-badge" :class="progressInfo.stage">{{ progressInfo.stage }}</span>
            </div>
            <div class="task-msg">{{ progressInfo.message }}</div>
            <div class="progress-bar-container" v-if="progressInfo.stage !== 'done' && progressInfo.stage !== 'error'">
              <div class="progress-bar" :style="{ width: progressInfo.total > 0 ? (progressInfo.downloaded / progressInfo.total * 100) + '%' : '100%' }" :class="{ 'indeterminate': progressInfo.total <= 0 }"></div>
            </div>
            <div class="task-actions">
              <button v-if="progressInfo.stage !== 'done' && progressInfo.stage !== 'error'" @click="cancelDownload" class="btn-cancel">取消</button>
              <button v-if="progressInfo.stage === 'error'" @click="startDownload" class="btn-primary">重试</button>
            </div>
          </div>

        </div>
      </transition>
    </main>

    <!-- Auth Modal -->
    <div class="modal-overlay" v-if="showAuthModal">
      <div class="modal">
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
:root {
  --bg-color: #F8F9FA;
  --card-bg: #FFFFFF;
  --input-bg: #F1F3F4;
  --primary: #8ED7EB;
  --primary-hover: #72C7DF;
  --text-main: #202427;
  --text-sub: #858B90;
  --border: #E0E4E8;
}

body {
  margin: 0;
  font-family: "Microsoft YaHei", -apple-system, BlinkMacSystemFont, sans-serif;
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
  background: var(--card-bg);
  box-shadow: 0 1px 2px rgba(0,0,0,0.05);
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

.main-content {
  flex: 1;
  padding: 40px 20px;
  max-width: 800px;
  margin: 0 auto;
  width: 100%;
  display: flex;
  flex-direction: column;
  transition: all 0.3s ease;
}

.main-content.has-result {
  padding-top: 20px;
}

.hero {
  text-align: center;
  margin-bottom: 40px;
}

.hero h1 {
  font-size: 32px;
  margin: 0 0 12px 0;
  font-weight: 600;
}

.hero p {
  color: var(--text-sub);
  margin: 0;
  font-size: 16px;
}

.search-box {
  display: flex;
  background: var(--card-bg);
  border-radius: 12px;
  padding: 8px;
  box-shadow: 0 4px 12px rgba(0,0,0,0.05);
  align-items: center;
  border: 1px solid var(--border);
  transition: box-shadow 0.3s;
}

.search-box:focus-within {
  box-shadow: 0 4px 16px rgba(142, 215, 235, 0.2);
  border-color: var(--primary);
}

.search-icon {
  color: var(--text-sub);
  margin: 0 12px;
}

.search-box input {
  flex: 1;
  border: none;
  outline: none;
  font-size: 16px;
  background: transparent;
  padding: 8px 0;
}

.search-box button {
  background: var(--primary);
  color: #000;
  border: none;
  padding: 10px 24px;
  border-radius: 8px;
  font-weight: 600;
  font-size: 15px;
  cursor: pointer;
  transition: background 0.2s;
}

.search-box button:hover:not(:disabled) {
  background: var(--primary-hover);
}

.search-box button:disabled {
  opacity: 0.7;
  cursor: not-allowed;
}

.support-text {
  text-align: center;
  color: var(--text-sub);
  font-size: 13px;
  margin-top: 16px;
}

.error-msg {
  color: #E53E3E;
  text-align: center;
  margin-top: 16px;
  background: #FED7D7;
  padding: 10px;
  border-radius: 8px;
}

.result-view {
  margin-top: 32px;
  display: flex;
  flex-direction: column;
  gap: 24px;
  flex: 1;
  overflow-y: auto;
  padding-bottom: 40px;
}

.media-card {
  background: var(--card-bg);
  border-radius: 12px;
  padding: 20px;
  border: 1px solid var(--border);
}

.media-info h2 {
  margin: 0 0 12px 0;
  font-size: 18px;
  line-height: 1.4;
}

.meta {
  display: flex;
  gap: 12px;
  color: var(--text-sub);
  font-size: 14px;
  align-items: center;
}

.tag {
  background: var(--input-bg);
  padding: 4px 8px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 600;
}

.options-section h3 {
  font-size: 16px;
  margin: 0 0 16px 0;
  font-weight: 600;
}

.quality-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
  gap: 16px;
}

.quality-card {
  background: var(--card-bg);
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 16px;
  cursor: pointer;
  position: relative;
  transition: all 0.2s;
}

.quality-card:hover {
  border-color: var(--primary);
}

.quality-card.active {
  border-color: var(--primary);
  background: #F0FAFD;
}

.q-title {
  font-weight: 600;
  font-size: 15px;
  margin-bottom: 4px;
}

.q-sub {
  color: var(--text-sub);
  font-size: 12px;
}

.check {
  position: absolute;
  top: 16px;
  right: 16px;
  color: var(--primary);
}

.image-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}

.img-preview {
  width: 80px;
  height: 80px;
  background: var(--input-bg);
  border-radius: 8px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  color: var(--text-sub);
  font-size: 12px;
  border: 1px solid var(--border);
}

.img-preview.more {
  font-size: 18px;
  font-weight: bold;
}

.download-bar {
  background: var(--card-bg);
  border-radius: 12px;
  padding: 20px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  border: 1px solid var(--border);
  margin-top: auto;
}

.dir-info {
  color: var(--text-sub);
  font-size: 14px;
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

.btn-primary:hover {
  background: var(--primary-hover);
}

.btn-cancel {
  background: #FFF;
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
  border: 1px solid var(--primary);
  box-shadow: 0 4px 12px rgba(142, 215, 235, 0.15);
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
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 70%;
}

.status-badge {
  font-size: 12px;
  padding: 4px 8px;
  border-radius: 12px;
  background: var(--input-bg);
  text-transform: capitalize;
}

.status-badge.done { background: #C6F6D5; color: #22543D; }
.status-badge.error { background: #FED7D7; color: #822727; }
.status-badge.muxing { background: #FEEBC8; color: #7B341E; }

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
  0% { transform: translateX(-100%); }
  100% { transform: translateX(330%); }
}

.task-actions {
  display: flex;
  justify-content: flex-end;
}

/* Modal */
.modal-overlay {
  position: fixed;
  top: 0; left: 0; right: 0; bottom: 0;
  background: rgba(0,0,0,0.4);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
}

.modal {
  background: var(--card-bg);
  padding: 32px;
  border-radius: 16px;
  width: 320px;
  text-align: center;
  box-shadow: 0 8px 24px rgba(0,0,0,0.1);
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

/* Transitions */
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.3s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
