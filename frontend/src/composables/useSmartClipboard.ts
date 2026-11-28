import { onMounted, onUnmounted, ref, watch, type Ref } from "vue";
import { isMyGo } from "mygo-runtime";
import { DesktopService, events } from "../mygo";
import {
  createSmartClipboardController,
  readClipboardText,
  type AutoDetectTrigger,
  type FocusTriggerSignal,
} from "../utils/clipboard";

export interface UseSmartClipboardParams {
  autoDetectEnabled: Ref<boolean>;
  currentView: Ref<"home" | "downloads">;
  inputUrl: Ref<string>;
  isBusy: Ref<boolean>;
  onClearError?: () => void;
}

export const useSmartClipboard = (params: UseSmartClipboardParams) => {
  const autoFillNotice = ref("");
  let noticeTimer: ReturnType<typeof setTimeout> | undefined;

  const clearAutoFillNotice = () => {
    autoFillNotice.value = "";
    if (noticeTimer !== undefined) {
      clearTimeout(noticeTimer);
      noticeTimer = undefined;
    }
  };

  const showAutoFillNotice = () => {
    clearAutoFillNotice();
    autoFillNotice.value = "已识别剪贴板中的链接";
    noticeTimer = setTimeout(() => {
      autoFillNotice.value = "";
      noticeTimer = undefined;
    }, 3600);
  };

  const controller = createSmartClipboardController({
    isEnabled: () => params.autoDetectEnabled.value,
    isHome: () => params.currentView.value === "home",
    isBusy: () => params.isBusy.value,
    getInput: () => params.inputUrl.value,
    setInput: (value) => {
      params.onClearError?.();
      params.inputUrl.value = value;
    },
    readClipboard: () => readClipboardText(),
    getSignalState: isMyGo() ? () => DesktopService.getClipboardSignalState() : undefined,
    onAutoFilled: () => showAutoFillNotice(),
  });

  const triggerCheck = (trigger: AutoDetectTrigger, signal?: FocusTriggerSignal) => {
    void controller.check(trigger, signal);
  };

  const markUserCleared = () => {
    clearAutoFillNotice();
    controller.markUserCleared();
  };

  const markManualInput = (text: string) => {
    clearAutoFillNotice();
    controller.markHandledText(text);
  };

  let offSignal: (() => void) | undefined;

  const onWindowFocus = () => {
    // 浏览器开发模式或原生事件未送达时的轻量补充触发（受控制器去重与守卫保护）。
    if (!isMyGo()) {
      triggerCheck("focus");
    }
  };

  watch(params.currentView, (next, prev) => {
    if (next === "home" && prev && prev !== "home") {
      triggerCheck("return-home");
    } else if (next !== "home") {
      clearAutoFillNotice();
    }
  });

  watch(params.isBusy, (busy) => {
    if (busy) clearAutoFillNotice();
  });

  onMounted(() => {
    if (isMyGo()) {
      offSignal = events.clipboardSignal.on((signal) => {
        triggerCheck("focus", signal);
      });
    }
    if (typeof window !== "undefined") {
      window.addEventListener("focus", onWindowFocus);
    }
    triggerCheck("startup");
  });

  onUnmounted(() => {
    clearAutoFillNotice();
    offSignal?.();
    if (typeof window !== "undefined") {
      window.removeEventListener("focus", onWindowFocus);
    }
  });

  return {
    autoFillNotice,
    clearAutoFillNotice,
    triggerCheck,
    markUserCleared,
    markManualInput,
  };
};
