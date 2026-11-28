import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(__dirname, "..");

const vite = await createServer({ root: "frontend", server: { middlewareMode: true, ws: false } });
try {
  const { cleanSupportedURL, extractSingleSupportedURL, isAutoDetectableMediaURL } =
    await vite.ssrLoadModule("/src/utils/url.ts");
  const {
    readClipboardText,
    fingerprintClipboardText,
    createSmartClipboardController,
  } = await vite.ssrLoadModule("/src/utils/clipboard.ts");

  // 1. 单链接提取、分享文案清洗与多链接拒绝
  assert.equal(
    cleanSupportedURL("https://www.bilibili.com/video/BV1xx411c7mD/?spm_id_from=333.1007&p=2#reply"),
    "https://www.bilibili.com/video/BV1xx411c7mD/?p=2",
  );
  assert.equal(
    cleanSupportedURL("https://www.youtube.com/watch?v=jNQXAC9IVRw&list=PL123&si=track"),
    "https://www.youtube.com/watch?v=jNQXAC9IVRw",
  );
  assert.equal(
    cleanSupportedURL("https://youtu.be/jNQXAC9IVRw?si=track123"),
    "https://youtu.be/jNQXAC9IVRw",
  );
  assert.equal(
    cleanSupportedURL("https://demo.lofter.com/post/1234_5678?act=share"),
    "https://demo.lofter.com/post/1234_5678",
  );

  // 单个分享文案允许提取并清洗
  assert.equal(
    extractSingleSupportedURL("【推荐视频】快来看 https://www.bilibili.com/video/BV1xx411c7mD/?spm_id_from=333！超精彩"),
    "https://www.bilibili.com/video/BV1xx411c7mD/",
  );
  assert.equal(
    extractSingleSupportedURL("分享一条 Shorts https://www.youtube.com/shorts/ljj8nhLYt6A?feature=share 给你"),
    "https://www.youtube.com/shorts/ljj8nhLYt6A",
  );
  assert.equal(
    extractSingleSupportedURL("LOFTER 图集 https://demo.lofter.com/post/1234_5678?from=app"),
    "https://demo.lofter.com/post/1234_5678",
  );

  // 同一链接重复出现（仅追踪参数不同）归并后仍为唯一链接
  assert.equal(
    extractSingleSupportedURL("https://youtu.be/jNQXAC9IVRw?si=aaa 和 https://youtu.be/jNQXAC9IVRw?si=bbb"),
    "https://youtu.be/jNQXAC9IVRw",
  );

  // 多个不同有效链接时绝不擅自选择
  assert.equal(
    extractSingleSupportedURL(
      "第一个 https://youtu.be/jNQXAC9IVRw 第二个 https://www.bilibili.com/video/BV1xx411c7mD",
    ),
    "",
  );
  assert.equal(
    extractSingleSupportedURL("https://youtu.be/jNQXAC9IVRw https://youtu.be/aqz-KE-bpKQ"),
    "",
  );

  // 普通文本、站点首页、不支持域名、仿冒域名一律忽略
  for (const text of [
    "",
    "只是一段普通文本，没有任何链接",
    "https://www.bilibili.com/",
    "https://www.lofter.com/",
    "https://example.com/watch?v=jNQXAC9IVRw",
    "https://youtube.com.evil.org/watch?v=jNQXAC9IVRw",
  ]) {
    assert.equal(extractSingleSupportedURL(text), "", `should ignore: ${text}`);
  }
  assert.equal(isAutoDetectableMediaURL("https://www.bilibili.com/bangumi/play/ep12345"), true);

  // 2. 桌面模式仅走 Go IPC（不调用 Web Clipboard API）；浏览器开发模式走兼容路径
  let webCalled = 0;
  let nativeCalled = 0;
  const desktopOK = await readClipboardText({
    isDesktop: () => true,
    readNative: async () => {
      nativeCalled++;
      return { status: "ok", text: "https://youtu.be/jNQXAC9IVRw", message: "" };
    },
    readBrowser: async () => {
      webCalled++;
      return "should-not-use";
    },
  });
  assert.deepEqual(desktopOK, { status: "ok", text: "https://youtu.be/jNQXAC9IVRw", message: "" });
  assert.equal(nativeCalled, 1);
  assert.equal(webCalled, 0);

  // 桌面模式非文本 / 空 / 占用 / 失败状态精确透传且不降级到 navigator.clipboard
  for (const sample of [
    { status: "empty", text: "", message: "剪贴板为空" },
    { status: "non-text", text: "", message: "剪贴板当前不是文本内容" },
    { status: "busy", text: "", message: "剪贴板暂时被其他程序占用，请稍后重试或按 Ctrl+V 粘贴" },
    { status: "error", text: "", message: "无法读取剪贴板，请使用 Ctrl+V 手动粘贴" },
  ]) {
    const res = await readClipboardText({
      isDesktop: () => true,
      readNative: async () => sample,
      readBrowser: async () => {
        webCalled++;
        return "nope";
      },
    });
    assert.deepEqual(res, sample);
  }
  assert.equal(webCalled, 0);

  // 浏览器开发模式保留 Web Clipboard 兼容
  const browserOK = await readClipboardText({
    isDesktop: () => false,
    readBrowser: async () => {
      webCalled++;
      return "https://www.bilibili.com/video/BV1xx411c7mD";
    },
  });
  assert.equal(browserOK.status, "ok");
  assert.equal(browserOK.text, "https://www.bilibili.com/video/BV1xx411c7mD");
  assert.equal(webCalled, 1);

  // 3. 智能自动识别状态机全场景（纯注入，不碰真实剪贴板）
  let enabled = false;
  let home = true;
  let busy = false;
  let input = "";
  let clipboardSeq = 1;
  let currentClipboard = { status: "ok", text: "推荐 https://youtu.be/jNQXAC9IVRw?si=abc 很棒", message: "" };
  let readCount = 0;
  const filledEvents = [];

  const ctrl = createSmartClipboardController({
    isEnabled: () => enabled,
    isHome: () => home,
    isBusy: () => busy,
    getInput: () => input,
    setInput: (v) => {
      input = v;
    },
    readClipboard: async () => {
      readCount++;
      return currentClipboard;
    },
    getSignalState: async () => ({ seq: clipboardSeq, watching: true }),
    onAutoFilled: (u) => filledEvents.push(u),
  });

  // 3.1 关闭开关时：绝不读取剪贴板
  assert.deepEqual(await ctrl.check("startup"), { filled: false, url: "" });
  assert.equal(readCount, 0);

  // 3.2 开启并首次检查：提取清洗后链接，仅填入输入框
  enabled = true;
  assert.deepEqual(await ctrl.check("enable"), {
    filled: true,
    url: "https://youtu.be/jNQXAC9IVRw",
  });
  assert.equal(input, "https://youtu.be/jNQXAC9IVRw");
  assert.equal(readCount, 1);
  assert.equal(filledEvents.length, 1);

  // 3.3 用户主动清空后，同一份剪贴板内容切回前台不得重复填入
  input = "";
  ctrl.markUserCleared();
  assert.deepEqual(
    await ctrl.check("focus", { seq: clipboardSeq, dirty: false, watching: true }),
    { filled: false, url: "" },
  );
  assert.equal(readCount, 1, "unchanged seq must skip reading clipboard");
  // 即便 seq 变化但内容相同，指纹去重也阻止重复填入
  clipboardSeq++;
  assert.deepEqual(
    await ctrl.check("focus", { seq: clipboardSeq, dirty: true, watching: true }),
    { filled: false, url: "" },
  );
  assert.equal(input, "");
  assert.equal(readCount, 2);

  // 3.4 输入框已有用户内容时，绝不覆盖
  input = "https://custom-user-input";
  clipboardSeq++;
  currentClipboard = { status: "ok", text: "https://www.bilibili.com/video/BV1xx411c7mD?spm_id_from=1", message: "" };
  assert.deepEqual(
    await ctrl.check("focus", { seq: clipboardSeq, dirty: true, watching: true }),
    { filled: false, url: "" },
  );
  assert.equal(input, "https://custom-user-input");
  assert.equal(readCount, 2, "non-empty input must not even read clipboard");

  // 3.5 不在首页时（如在下载页）不触发自动填入；返回首页时检查尚未处理的变化
  input = "";
  home = false;
  assert.deepEqual(
    await ctrl.check("focus", { seq: clipboardSeq, dirty: true, watching: true }),
    { filled: false, url: "" },
  );
  assert.equal(readCount, 2);

  home = true;
  assert.deepEqual(await ctrl.check("return-home"), {
    filled: true,
    url: "https://www.bilibili.com/video/BV1xx411c7mD",
  });
  assert.equal(input, "https://www.bilibili.com/video/BV1xx411c7mD");
  assert.equal(readCount, 3);

  // 3.6 解析中 / 资源选择中时绝不触发
  input = "";
  busy = true;
  clipboardSeq++;
  currentClipboard = { status: "ok", text: "https://demo.lofter.com/post/1234_5678", message: "" };
  assert.deepEqual(
    await ctrl.check("focus", { seq: clipboardSeq, dirty: true, watching: true }),
    { filled: false, url: "" },
  );
  assert.equal(readCount, 3);

  // 3.7 非文本 / 占用 / 失败时静默处理，不抛错、不填入
  busy = false;
  for (const status of ["empty", "non-text", "busy", "error"]) {
    clipboardSeq++;
    currentClipboard = { status, text: "", message: "err" };
    assert.deepEqual(
      await ctrl.check("focus", { seq: clipboardSeq, dirty: true, watching: true }),
      { filled: false, url: "" },
    );
    assert.equal(input, "");
  }

  // 3.8 指纹不包含剪贴板原文
  const fp = fingerprintClipboardText("https://youtu.be/jNQXAC9IVRw");
  assert.ok(!fp.includes("jNQXAC9IVRw") && !fp.includes("youtu.be"));

  // 4. 源码契约检查：LinkInput 不再直接调用 navigator.clipboard.readText，保留 @paste 支持 Ctrl+V
  const linkInputSrc = readFileSync(path.join(root, "src/components/home/LinkInput.vue"), "utf8");
  assert.doesNotMatch(linkInputSrc, /navigator\.clipboard/, "LinkInput must route through utils/clipboard instead of calling navigator.clipboard directly");
  assert.match(linkInputSrc, /readClipboardText/, "LinkInput must use readClipboardText");
  assert.match(linkInputSrc, /@paste=/, "LinkInput must preserve native Ctrl+V paste handling");

  console.log("剪贴板原生读取、单链接提取与智能自动识别契约测试通过");
} finally {
  await vite.close();
}
