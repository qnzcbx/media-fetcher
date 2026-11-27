from pathlib import Path
from playwright.sync_api import sync_playwright, expect

ROOT = Path(__file__).resolve().parents[2]
OUTPUT = ROOT / "notes" / "youtube-ui-screenshots"
OUTPUT.mkdir(parents=True, exist_ok=True)

# Mock IPC 数据（仅用于 UI 交互、主题与视觉截图验收，与真实端到端下载验证明确区分）
MOCK = r"""
let progressCb = null;
const coverData = "data:image/svg+xml;base64," + btoa('<svg xmlns="http://www.w3.org/2000/svg" width="320" height="180"><rect width="320" height="180" fill="#182230"/><circle cx="160" cy="90" r="34" fill="#ff0000"/><polygon points="150,74 150,106 178,90" fill="#fff"/></svg>');
const ytMedia = {
  episodes: [],
  sourceUrl: "https://www.youtube.com/watch?v=aqz-KE-bpKQ",
  seasonTitle: "",
  episodeTitle: "",
  siteId: "youtube",
  id: "aqz-KE-bpKQ",
  title: "Big Buck Bunny 60fps 4K - Official Blender Foundation Short Film",
  author: "Blender",
  coverUrl: "https://i.ytimg.com/vi/aqz-KE-bpKQ/maxresdefault.jpg",
  isAnonymous: true,
  usedAccountId: "",
  videoStreams: [
    { id: 315, quality_group_key: "2160p-60fps-SDR", stream_key: "315", container: "webm", quality_label: "4K 60帧", width: 3840, height: 2160, codec: "vp9", bandwidth: 17174000, url: "https://mock/315", frame_rate: "60", size: 1362269481, need_auth: false },
    { id: 401, quality_group_key: "2160p-60fps-SDR", stream_key: "401", container: "mp4", quality_label: "4K 60帧", width: 3840, height: 2160, codec: "av01.0.13M.08", bandwidth: 8981000, url: "https://mock/401", frame_rate: "60", size: 712445280, need_auth: false },
    { id: 299, quality_group_key: "1080p-60fps-SDR", stream_key: "299", container: "mp4", quality_label: "1080P 60帧", width: 1920, height: 1080, codec: "avc1.64002A", bandwidth: 6933000, url: "https://mock/299", frame_rate: "60", size: 257619653, need_auth: false },
    { id: 303, quality_group_key: "1080p-60fps-SDR", stream_key: "303", container: "webm", quality_label: "1080P 60帧", width: 1920, height: 1080, codec: "vp9", bandwidth: 4652000, url: "https://mock/303", frame_rate: "60", size: 168736189, need_auth: false },
    { id: 399, quality_group_key: "1080p-60fps-SDR", stream_key: "399", container: "mp4", quality_label: "1080P 60帧", width: 1920, height: 1080, codec: "av01.0.09M.08", bandwidth: 4492000, url: "https://mock/399", frame_rate: "60", size: 124386876, need_auth: false },
    { id: 298, quality_group_key: "720p-60fps-SDR", stream_key: "298", container: "mp4", quality_label: "720P 60帧", width: 1280, height: 720, codec: "avc1.4D4020", bandwidth: 4206000, url: "https://mock/298", frame_rate: "60", size: 150524867, need_auth: false },
  ],
  audioStreams: [
    { id: 140, stream_key: "140", container: "m4a", kind: "standard", quality: "129Kbps AAC", codec: "mp4a.40.2", bandwidth: 129481, url: "https://mock/140", size: 10271496 },
    { id: 251, stream_key: "251", container: "webm", kind: "standard", quality: "129Kbps Opus", codec: "opus", bandwidth: 128612, url: "https://mock/251", size: 10202210 },
    { id: 328, stream_key: "328", container: "m4a", kind: "dolby", quality: "384Kbps 5.1 Dolby E-AC-3", codec: "ec-3", bandwidth: 384105, url: "https://mock/328", size: 30468811 },
  ],
  images: [],
};

let historyRecords = [];
export const DesktopService = new Proxy({}, {
  get(_, key) {
    if (key === "getAccounts") return async () => [];
    if (key === "downloadHistory") return async () => historyRecords;
    if (key === "getDownloadFileStates") return async (ids) => ids.map((id) => ({ id, status: "available", canRecycle: true, message: "" }));
    if (key === "fetchPreviewBase64") return async () => coverData;
    if (key === "validateDownloadDirectory") return async () => "C:/Downloads";
    if (key === "getDefaultDownloadDirectory") return async () => "C:/Downloads";
    if (key === "extract") return async () => ytMedia;
    if (key === "startDownload") return async (params) => {
      const ext = params.VideoCodec.startsWith("vp9") && params.AudioKey === "251" ? "webm" : params.VideoCodec.startsWith("vp9") || params.AudioKey === "251" ? "mkv" : "mp4";
      const rec = {
        id: params.TaskID + "-media",
        taskId: params.TaskID,
        siteId: "youtube",
        kind: params.Mode === "audio" ? "audio" : "video",
        title: ytMedia.title,
        sourceUrl: ytMedia.sourceUrl,
        finalPath: `C:/Downloads/Big Buck Bunny 60fps 4K.${ext}`,
        thumbnail: "",
        createdAt: new Date().toISOString(),
        completedAt: new Date().toISOString(),
        status: "done",
        resourceId: params.VideoID >= 0 ? params.VideoID : params.AudioID,
        codec: params.VideoCodec || "opus",
        fileSize: 178938399,
        format: ext,
        durationSeconds: 635,
      };
      historyRecords = [rec, ...historyRecords];
      setTimeout(() => {
        progressCb?.({ taskId: params.TaskID, itemId: "media", stage: "done", record: rec, downloaded: 100, total: 100, overallDownloaded: 100, overallTotal: 100, message: "", finalPath: rec.finalPath });
        progressCb?.({ taskId: params.TaskID, itemId: "", stage: "finished", downloaded: 0, total: 0, overallDownloaded: 0, overallTotal: 0, message: "", finalPath: "" });
      }, 60);
    };
    return async () => [];
  },
});
export const events = {
  progress: {
    on: (cb) => {
      progressCb = cb;
      return () => { progressCb = null; };
    },
  },
};
"""

with sync_playwright() as p:
    browser = p.chromium.launch(headless=True)
    for theme in ["light", "dark"]:
        page = browser.new_page(viewport={"width": 960, "height": 680})
        page.route("**/src/mygo.ts", lambda route: route.fulfill(status=200, content_type="text/javascript", body=MOCK))
        page.goto("http://127.0.0.1:5178")
        page.wait_for_load_state("networkidle")
        page.evaluate("(theme) => document.documentElement.dataset.theme = theme", theme)
        page.wait_for_timeout(150)

        # 验证首页支持站点包含 YouTube
        expect(page.locator(".support-text")).to_contain_text("YouTube")
        page.get_by_role("textbox", name="你的链接").fill("https://www.youtube.com/watch?v=aqz-KE-bpKQ")
        page.get_by_role("button", name="下载", exact=True).click()

        # 1. 完整视频资源选择（3 个分组卡片，而非 6 个重复 itag 卡片）
        expect(page.locator(".media-card")).to_have_count(3)
        # 默认最高画质 4K 60帧（VP9）+ AAC -> 无损 MKV 预览
        expect(page.get_by_role("button", name="下载 MKV", exact=True)).to_be_visible()
        # 切换至 720P 60帧（仅 AVC）+ AAC -> 下载 MP4，再切到 1080P 60帧 保持 AVC -> 下载 MP4
        page.locator(".media-card").filter(has_text="720P 60帧").click()
        expect(page.get_by_role("button", name="下载 MP4", exact=True)).to_be_visible()
        page.locator(".media-card").filter(has_text="1080P 60帧").click()
        expect(page.get_by_role("button", name="下载 MP4", exact=True)).to_be_visible()
        page.wait_for_timeout(200)
        page.screenshot(path=str(OUTPUT / f"{theme}-youtube-complete-mock.png"))

        # 2. 高级设置浮层（切换 VP9 + Opus -> 动态变为 下载 WebM）
        page.get_by_role("button", name="高级设置").click()
        expect(page.locator(".advanced-panel")).to_be_visible()
        page.get_by_role("button", name="VP9", exact=True).click()
        expect(page.get_by_role("button", name="下载 MKV", exact=True)).to_be_visible()
        page.get_by_role("combobox", name="音质", exact=True).click()
        page.get_by_role("option", name="129Kbps Opus", exact=True).click()
        expect(page.get_by_role("button", name="下载 WebM", exact=True)).to_be_visible()
        page.wait_for_timeout(200)
        page.screenshot(path=str(OUTPUT / f"{theme}-youtube-advanced-mock.png"))
        page.get_by_role("button", name="关闭高级设置", exact=True).click()

        # 3. 仅视频模式（VP9 -> 下载 WebM）
        page.get_by_role("button", name="仅视频", exact=True).click()
        expect(page.get_by_role("button", name="下载 WebM", exact=True)).to_be_visible()
        page.wait_for_timeout(200)
        page.screenshot(path=str(OUTPUT / f"{theme}-youtube-video-only-mock.png"))

        # 4. 仅音频模式
        page.get_by_role("button", name="仅音频", exact=True).click()
        page.get_by_role("button", name="原始", exact=True).click()
        expect(page.get_by_role("button", name="下载原始音频", exact=True)).to_be_visible()
        page.wait_for_timeout(200)
        page.screenshot(path=str(OUTPUT / f"{theme}-youtube-audio-only-mock.png"))

        # 5. 切回完整视频并触发下载 -> 验收下载完成后列表（打开详细管理页展示完整下载列表）
        page.get_by_role("button", name="完整视频", exact=True).click()
        page.get_by_role("button", name="下载 WebM", exact=True).click()
        expect(page.locator(".resource-modal")).to_have_count(0)
        expect(page.get_by_role("button", name="打开详细管理页")).to_be_visible()
        page.get_by_role("button", name="打开详细管理页").click()
        expect(page.get_by_text("Big Buck Bunny 60fps 4K")).to_be_visible()
        page.wait_for_timeout(300)
        page.screenshot(path=str(OUTPUT / f"{theme}-youtube-downloads-mock.png"))
        page.close()
    browser.close()
print("YouTube Playwright UI verification passed (light + dark, 10 screenshots saved)")
