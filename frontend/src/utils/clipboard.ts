import { isMyGo } from "mygo-runtime";
import {
  DesktopService,
  type ClipboardReadResult,
  type ClipboardSignalState,
} from "../mygo";
import { extractSingleSupportedURL } from "./url";

export type ClipboardReadStatus = "ok" | "empty" | "non-text" | "busy" | "error";

export interface NormalizedClipboardResult {
  status: ClipboardReadStatus;
  text: string;
  message: string;
}

export interface ClipboardReaderDeps {
  isDesktop?: () => boolean;
  readNative?: () => Promise<ClipboardReadResult>;
  readBrowser?: () => Promise<string>;
}

const DEFAULT_ERROR_MESSAGE = "无法读取剪贴板，请使用 Ctrl+V 手动粘贴";

const normalizeStatus = (status: string): ClipboardReadStatus => {
  switch (status) {
    case "ok":
    case "empty":
    case "non-text":
    case "busy":
    case "error":
      return status;
    default:
      return "error";
  }
};

// 正式桌面端始终通过 MyGo Go IPC 读取剪贴板，不依赖 WebView2 navigator.clipboard；
// 仅在浏览器开发模式保留 Web Clipboard API 兼容路径。
export const readClipboardText = async (
  deps: ClipboardReaderDeps = {},
): Promise<NormalizedClipboardResult> => {
  const desktop = (deps.isDesktop ?? isMyGo)();
  if (desktop) {
    try {
      const res = await (deps.readNative ?? DesktopService.readClipboardText)();
      const status = normalizeStatus(res?.status ?? "error");
      if (status === "ok" && res.text && res.text.trim()) {
        return { status: "ok", text: res.text, message: "" };
      }
      if (status === "ok") {
        return { status: "empty", text: "", message: "剪贴板为空" };
      }
      return {
        status,
        text: "",
        message: res?.message || DEFAULT_ERROR_MESSAGE,
      };
    } catch {
      return { status: "error", text: "", message: DEFAULT_ERROR_MESSAGE };
    }
  }

  try {
    const readBrowser =
      deps.readBrowser ??
      (async () => {
        if (typeof navigator === "undefined" || !navigator.clipboard?.readText) {
          throw new Error("web clipboard unavailable");
        }
        return navigator.clipboard.readText();
      });
    const text = await readBrowser();
    if (!text || !text.trim()) {
      return { status: "empty", text: "", message: "剪贴板为空" };
    }
    return { status: "ok", text, message: "" };
  } catch {
    return { status: "error", text: "", message: DEFAULT_ERROR_MESSAGE };
  }
};

// 仅在内存中生成不可逆指纹，用于避免重复填入同一份剪贴板内容，绝不保留剪贴板原文。
export const fingerprintClipboardText = (text: string): string => {
  let h1 = 0x811c9dc5;
  let h2 = 0x9e3779b1;
  for (let i = 0; i < text.length; i++) {
    const code = text.charCodeAt(i);
    h1 ^= code;
    h1 = Math.imul(h1, 0x01000193);
    h2 ^= code + i;
    h2 = Math.imul(h2, 0x85ebca6b);
  }
  return `${text.length}:${(h1 >>> 0).toString(16)}:${(h2 >>> 0).toString(16)}`;
};

export type AutoDetectTrigger = "startup" | "focus" | "return-home" | "enable";

export interface FocusTriggerSignal {
  seq?: number;
  dirty?: boolean;
  watching?: boolean;
}

export interface SmartClipboardOptions {
  isEnabled: () => boolean;
  isHome: () => boolean;
  isBusy: () => boolean;
  getInput: () => string;
  setInput: (value: string) => void;
  readClipboard: () => Promise<NormalizedClipboardResult>;
  getSignalState?: () => Promise<ClipboardSignalState>;
  onAutoFilled?: (url: string) => void;
}

export interface SmartClipboardController {
  check: (trigger: AutoDetectTrigger, signal?: FocusTriggerSignal) => Promise<{ filled: boolean; url: string }>;
  markHandledText: (text: string) => void;
  markUserCleared: () => void;
}

// 纯逻辑状态机，确保严格遵守全部自动填入前置条件与隐私边界，并支持测试注入。
export const createSmartClipboardController = (
  options: SmartClipboardOptions,
): SmartClipboardController => {
  let lastCheckedSeq: number | undefined;
  let lastHandledFingerprint = "";
  let inFlight = false;

  const canAutoDetect = () =>
    options.isEnabled() &&
    options.isHome() &&
    !options.isBusy() &&
    options.getInput().trim() === "";

  const markHandledText = (text: string) => {
    if (text && text.trim()) {
      lastHandledFingerprint = fingerprintClipboardText(text);
    }
  };

  const markUserCleared = () => {
    // 保留 lastHandledFingerprint，确保用户主动清空后不会重复填入同一份剪贴板内容。
    // 若用户清空前是自动填入的内容，其指纹已在填入时记录。
  };

  const check = async (
    trigger: AutoDetectTrigger,
    signal?: FocusTriggerSignal,
  ): Promise<{ filled: boolean; url: string }> => {
    if (inFlight || !canAutoDetect()) {
      return { filled: false, url: "" };
    }

    let currentSeq = signal?.seq;
    let watching = signal?.watching;

    if (trigger === "focus" && watching && !signal?.dirty && currentSeq !== undefined && currentSeq === lastCheckedSeq) {
      return { filled: false, url: "" };
    }

    if ((trigger === "return-home" || trigger === "startup" || trigger === "enable") && options.getSignalState) {
      try {
        const state = await options.getSignalState();
        currentSeq = state.seq;
        watching = state.watching;
        if (trigger === "return-home" && watching && currentSeq === lastCheckedSeq) {
          return { filled: false, url: "" };
        }
      } catch {
        // 无法获取序号时仍允许单次受保护读取（由指纹去重兜底）。
      }
    }

    if (!canAutoDetect()) {
      return { filled: false, url: "" };
    }

    inFlight = true;
    try {
      const res = await options.readClipboard();
      if (currentSeq !== undefined) {
        lastCheckedSeq = currentSeq;
      }
      if (res.status !== "ok" || !res.text.trim()) {
        return { filled: false, url: "" };
      }

      const fp = fingerprintClipboardText(res.text);
      if (fp === lastHandledFingerprint) {
        return { filled: false, url: "" };
      }

      const candidate = extractSingleSupportedURL(res.text);
      lastHandledFingerprint = fp;
      if (!candidate) {
        return { filled: false, url: "" };
      }

      // 异步读取返回后再次校验状态，避免覆盖用户期间输入或弹窗状态。
      if (!canAutoDetect()) {
        return { filled: false, url: "" };
      }

      options.setInput(candidate);
      options.onAutoFilled?.(candidate);
      return { filled: true, url: candidate };
    } catch {
      // 自动识别失败一律静默处理，不展示红色错误条。
      return { filled: false, url: "" };
    } finally {
      inFlight = false;
    }
  };

  return { check, markHandledText, markUserCleared };
};
