from pathlib import Path
from playwright.sync_api import sync_playwright, expect

ROOT = Path(__file__).resolve().parents[2]
OUTPUT = ROOT / "notes" / "03.7B.3-ui-screenshots"
OUTPUT.mkdir(parents=True, exist_ok=True)

# 03.7B.3 交互验收 Mock IPC：覆盖首页更多菜单、下载管理页长列表滚动条与菜单互斥、慢速/快速/失败解析遮罩。
MOCK = r"""
let progressCb = null;
const coverSvg = (color, label) =>
  "data:image/svg+xml;base64," +
  btoa(`<svg xmlns="http://www.w3.org/2000/svg" width="320" height="180"><rect width="320" height="180" fill="${color}"/><text x="160" y="98" text-anchor="middle" fill="#fff" font-size="20" font-family="sans-serif">${label}</text></svg>`);

const palette = ["#243447", "#3a2d4f", "#1f4238", "#4d3228", "#2d3b55", "#3d4a3a"];
const historyRecords = Array.from({ length: 18 }, (_, i) => {
  const isImage = i % 5 === 4;
  const isAudio = i % 7 === 6;
  const kind = isImage ? "image" : isAudio ? "audio" : "video";
  const ext = isImage ? "png" : isAudio ? "opus" : i % 2 === 0 ? "webm" : "mp4";
  const siteId = isImage ? "lofter" : i % 3 === 0 ? "youtube" : "bilibili";
  return {
    id: `rec-${i + 1}`,
    taskId: `000000000000000000000000000000${String(i + 1).padStart(2, "0")}`,
    siteId,
    kind,
    title: `示例下载条目 #${i + 1} - ${siteId.toUpperCase()}`,
    sourceUrl: siteId === "youtube" ? `https://www.youtube.com/watch?v=aqz-KE-bpK${i % 10}` : "https://www.bilibili.com/video/BV1xx411c7mD",
    finalPath: `C:/Downloads/示例下载条目_${i + 1}.${ext}`,
    thumbnail: `thumb-${i + 1}`,
    createdAt: new Date(Date.now() - i * 60000).toISOString(),
    completedAt: new Date(Date.now() - i * 60000).toISOString(),
    status: "done",
    resourceId: i + 1,
    codec: isImage ? "" : isAudio ? "opus" : "vp9",
    fileSize: (i + 2) * 4_500_000,
    format: ext,
    durationSeconds: isImage ? 0 : 120 + i * 19,
  };
});

window.__openRecordCalls = [];
window.__extractCalls = 0;

const sampleMedia = (siteId) => ({
  episodes: [],
  sourceUrl: siteId === "youtube" ? "https://www.youtube.com/watch?v=aqz-KE-bpKQ" : "https://www.bilibili.com/video/BV1xx411c7mD",
  seasonTitle: "",
  episodeTitle: "",
  siteId,
  id: siteId === "youtube" ? "aqz-KE-bpKQ" : "BV1xx411c7mD",
  title: siteId === "youtube" ? "Big Buck Bunny 60fps 4K" : "Bilibili Fast Sample",
  author: "Tester",
  coverUrl: "https://mock/cover.jpg",
  isAnonymous: true,
  usedAccountId: "",
  videoStreams: [
    { id: 299, quality_group_key: "1080p-60fps-SDR", stream_key: "299", container: "mp4", quality_label: "1080P 60帧", width: 1920, height: 1080, codec: "avc1.64002A", bandwidth: 6900000, url: "https://mock/299", frame_rate: "60", size: 50000000, need_auth: false },
  ],
  audioStreams: [
    { id: 140, stream_key: "140", container: "m4a", kind: "standard", quality: "129Kbps AAC", codec: "mp4a.40.2", bandwidth: 129000, url: "https://mock/140", size: 5000000 },
  ],
  images: [],
});

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

export const DesktopService = new Proxy({}, {
  get(_, key) {
    if (key === "getAccounts") return async () => [];
    if (key === "downloadHistory") return async () => historyRecords;
    if (key === "getDownloadFileStates") return async (ids) => ids.map((id) => ({ id, status: "available", canRecycle: true, message: "" }));
    if (key === "galleryThumbnail") return async (id) => {
      const idx = Number(id.replace("rec-", "")) || 1;
      return coverSvg(palette[idx % palette.length], `#${idx}`);
    };
    if (key === "fetchPreviewBase64") return async () => coverSvg("#1d2b3a", "MEDIA");
    if (key === "validateDownloadDirectory") return async () => "C:/Downloads";
    if (key === "getDefaultDownloadDirectory") return async () => "C:/Downloads";
    if (key === "openDownloadRecord") return async (id, action) => {
      window.__openRecordCalls.push({ id, action });
    };
    if (key === "clearExtractSnapshot") return async () => {};
    if (key === "extract") return async (url) => {
      window.__extractCalls += 1;
      if (url.includes("fail0000000")) {
        await sleep(950);
        throw new Error("youtube: 该视频已删除或不可用");
      }
      if (url.includes("youtube.com")) {
        await sleep(1400);
        return sampleMedia("youtube");
      }
      // Bilibili 快速解析（< 500ms，遮罩不应闪现）
      await sleep(120);
      return sampleMedia("bilibili");
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


def run():
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        for theme in ["light", "dark"]:
            page = browser.new_page(viewport={"width": 960, "height": 680})
            page.route("**/src/mygo.ts", lambda route: route.fulfill(status=200, content_type="text/javascript", body=MOCK))
            page.goto("http://127.0.0.1:5178")
            page.wait_for_load_state("networkidle")
            page.evaluate("(theme) => document.documentElement.dataset.theme = theme", theme)
            page.wait_for_timeout(200)

            # ------------------------------------------------------------------
            # P0-A: 首页「我的下载」卡片更多菜单：展开/再次点击关闭/互斥/外部点击不拦截/Escape 关闭
            # ------------------------------------------------------------------
            tiles = page.locator(".gallery .tile")
            expect(tiles).to_have_count(4)

            # 1) 点击卡片 1 更多按钮展开，再次点击关闭
            tiles.nth(0).locator(".more").click(force=True)
            expect(page.locator(".gallery .tile .actions")).to_have_count(1)
            tiles.nth(0).locator(".more").click(force=True)
            expect(page.locator(".gallery .tile .actions")).to_have_count(0)

            # 2) 展开卡片 1，再点击卡片 2 更多按钮 -> 互斥（关闭卡片 1 并展开卡片 2）
            tiles.nth(0).locator(".more").click(force=True)
            expect(tiles.nth(0).locator(".actions")).to_be_visible()
            tiles.nth(1).locator(".more").click(force=True)
            expect(tiles.nth(0).locator(".actions")).to_have_count(0)
            expect(tiles.nth(1).locator(".actions")).to_be_visible()
            expect(page.locator(".gallery .tile .actions")).to_have_count(1)
            page.screenshot(path=str(OUTPUT / f"{theme}-01-home-more-menu.png"))

            # 3) 点击菜单外部任意区域（例如标题空白区域）立即关闭
            page.locator(".brand-wordmark, h1, header").first.click(position={"x": 10, "y": 10})
            expect(page.locator(".gallery .tile .actions")).to_have_count(0)

            # 4) Escape 关闭
            tiles.nth(0).locator(".more").click(force=True)
            expect(page.locator(".gallery .tile .actions")).to_have_count(1)
            page.keyboard.press("Escape")
            expect(page.locator(".gallery .tile .actions")).to_have_count(0)

            # 5) 外部点击关闭不阻止其他正常操作（展开菜单后直接点“打开设置”按钮，菜单关闭且设置抽屉打开）
            tiles.nth(0).locator(".more").click(force=True)
            expect(page.locator(".gallery .tile .actions")).to_have_count(1)
            page.get_by_role("button", name="设置").click()
            expect(page.locator(".gallery .tile .actions")).to_have_count(0)
            expect(page.locator(".settings-header h2")).to_be_visible()
            page.locator(".settings-header").get_by_role("button", name="关闭").click()
            expect(page.locator(".settings-header h2")).to_have_count(0)

            # ------------------------------------------------------------------
            # P0-B + P1: 详细下载管理页长列表滚动条与更多菜单互斥/外部点击/滚动/筛选关闭
            # ------------------------------------------------------------------
            page.get_by_role("button", name="打开详细管理页").click()
            expect(page.locator(".downloads-view")).to_be_visible()
            page.wait_for_timeout(250)

            list_container = page.locator(".list-container")
            items = page.locator(".list-item")
            expect(items).to_have_count(18)

            # 确认长列表可滚动且具备稳定滚动条槽位（切换短列表时 .list 宽度不抖动）
            scroll_metrics = list_container.evaluate("""(el) => ({
                scrollHeight: el.scrollHeight,
                clientHeight: el.clientHeight,
                gutter: getComputedStyle(el).scrollbarGutter,
            })""")
            assert scroll_metrics["scrollHeight"] > scroll_metrics["clientHeight"], scroll_metrics
            assert "stable" in scroll_metrics["gutter"], scroll_metrics
            long_list_width = page.locator(".list").bounding_box()["width"]

            # 展开第 1 行菜单 -> 切到第 4 行菜单（非重叠区域，纯点击互斥）
            items.nth(0).locator(".more-btn").click()
            expect(items.nth(0).locator(".dropdown-menu")).to_be_visible()
            expect(page.locator(".dropdown-menu")).to_have_count(1)
            items.nth(3).locator(".more-btn").click()
            expect(items.nth(0).locator(".dropdown-menu")).to_have_count(0)
            expect(items.nth(3).locator(".dropdown-menu")).to_be_visible()
            expect(page.locator(".dropdown-menu")).to_have_count(1)
            page.screenshot(path=str(OUTPUT / f"{theme}-02-downloads-list-menu-scrollbar.png"))

            # 点击外部区域立即关闭
            page.locator(".downloads-view h1").click()
            expect(page.locator(".dropdown-menu")).to_have_count(0)

            # 再次展开 -> 滚动列表立即关闭（同时验证滚动能力正常）
            items.nth(0).locator(".more-btn").click()
            expect(page.locator(".dropdown-menu")).to_have_count(1)
            list_container.evaluate("(el) => { el.scrollTop = 260; }")
            page.wait_for_timeout(120)
            expect(page.locator(".dropdown-menu")).to_have_count(0)
            assert list_container.evaluate("(el) => el.scrollTop") > 0

            # 滚回顶部 -> 执行菜单操作关闭菜单并触发对应动作
            list_container.evaluate("(el) => { el.scrollTop = 0; }")
            page.wait_for_timeout(80)
            items.nth(0).locator(".more-btn").click()
            items.nth(0).get_by_role("button", name="打开目录").click()
            expect(page.locator(".dropdown-menu")).to_have_count(0)
            calls = page.evaluate("() => window.__openRecordCalls")
            assert any(c["id"] == "rec-1" and c["action"] == "folder" for c in calls), calls

            # 展开菜单后点击“图片”筛选标签 -> 不阻止切换且菜单关闭，且短列表宽度不抖动
            items.nth(0).locator(".more-btn").click()
            expect(page.locator(".dropdown-menu")).to_have_count(1)
            page.get_by_role("button", name="图片", exact=True).click()
            expect(page.locator(".dropdown-menu")).to_have_count(0)
            page.wait_for_timeout(100)
            short_list_width = page.locator(".list").bounding_box()["width"]
            assert abs(long_list_width - short_list_width) < 1.5, (long_list_width, short_list_width)

            # 返回首页
            page.locator(".downloads-view .header .icon-btn").click()
            expect(page.locator(".main-content")).to_be_visible()
            page.wait_for_timeout(200)

            # ------------------------------------------------------------------
            # P2-A: 快速解析（Bilibili 120ms < 500ms）-> 遮罩不闪现，直接弹出资源选择器
            # ------------------------------------------------------------------
            page.get_by_role("textbox", name="你的链接").fill("https://www.bilibili.com/video/BV1xx411c7mD")
            page.get_by_role("button", name="下载", exact=True).click()
            page.wait_for_timeout(60)
            expect(page.locator(".parsing-overlay")).to_have_count(0)
            expect(page.locator(".resource-modal")).to_be_visible(timeout=3000)
            expect(page.locator(".parsing-overlay")).to_have_count(0)
            page.get_by_role("button", name="关闭", exact=True).click()
            expect(page.locator(".resource-modal")).to_have_count(0)

            # ------------------------------------------------------------------
            # P2-B: YouTube 慢速解析（1400ms > 500ms）-> 500ms 前不显示，500ms 后显示遮罩、防重复提交、成功后自动退出并弹出资源选择
            # ------------------------------------------------------------------
            calls_before = page.evaluate("() => window.__extractCalls")
            page.get_by_role("textbox", name="你的链接").fill("https://www.youtube.com/watch?v=aqz-KE-bpKQ")
            page.get_by_role("button", name="下载", exact=True).click()
            # 180ms 内不应显示（防闪烁延迟）
            page.wait_for_timeout(180)
            expect(page.locator(".parsing-overlay")).to_have_count(0)
            # 超过 500ms 后应展示遮罩与动态平台副文案
            expect(page.locator(".parsing-overlay")).to_be_visible(timeout=1500)
            expect(page.locator(".parsing-title")).to_have_text("正在全力解析")
            expect(page.locator(".parsing-subtitle")).to_have_text("正在获取 YouTube 的可用媒体资源")
            expect(page.get_by_role("textbox", name="你的链接")).to_be_disabled()
            page.wait_for_timeout(180)
            page.screenshot(path=str(OUTPUT / f"{theme}-03-youtube-parsing-overlay.png"))

            # 解析成功后遮罩退出并展示资源选择弹窗，且无重复提交
            expect(page.locator(".resource-modal")).to_be_visible(timeout=4000)
            expect(page.locator(".parsing-overlay")).to_have_count(0)
            assert page.evaluate("() => window.__extractCalls") == calls_before + 1
            page.get_by_role("button", name="关闭", exact=True).click()
            expect(page.locator(".resource-modal")).to_have_count(0)

            # ------------------------------------------------------------------
            # P2-C: 慢速解析失败（950ms）-> 遮罩显示后关闭，回显原有错误提示
            # ------------------------------------------------------------------
            page.get_by_role("textbox", name="你的链接").fill("https://www.youtube.com/watch?v=fail0000000")
            page.get_by_role("button", name="下载", exact=True).click()
            expect(page.locator(".parsing-overlay")).to_be_visible(timeout=1500)
            expect(page.locator(".main-content .error-msg")).to_contain_text("已删除或不可用", timeout=4000)
            expect(page.locator(".parsing-overlay")).to_have_count(0)
            expect(page.get_by_role("textbox", name="你的链接")).to_be_enabled()

            page.close()
        browser.close()
    print("03.7B.3 Playwright UI interaction verification passed (light + dark, 6 screenshots saved)")


if __name__ == "__main__":
    run()
