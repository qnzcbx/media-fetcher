import { onMounted, onUnmounted, ref, watch, type Ref } from "vue";
import { isMyGo, onWindowActive, windowActive } from "mygo-runtime";
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
  const isForeground = ref(
    isMyGo()
      ? windowActive()
      : typeof document === "undefined"
        ? true
        : document.visibilityState !== "hidden",
  );
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
    isForeground: () => isForeground.value,
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
  let offWindowActive: (() => void) | undefined;

  const onWindowFocus = () => {
    isForeground.value = true;
    if (!isMyGo()) {
      triggerCheck("focus");
    }
  };

  const onWindowBlur = () => {
    isForeground.value = false;
    controller.invalidate();
  };

  const onVisibilityChange = () => {
    if (typeof document !== "undefined" && document.visibilityState === "hidden") {
      isForeground.value = false;
      controller.invalidate();
    }
  };

  watch(params.currentView, (next, prev) => {
    if (next === "home" && prev && prev !== "home") {
      triggerCheck("return-home");
    } else if (next !== "home") {
      controller.invalidate();
      clearAutoFillNotice();
    }
  });

  watch(params.isBusy, (busy) => {
    if (busy) {
      controller.invalidate();
      clearAutoFillNotice();
    }
  });

  watch(params.autoDetectEnabled, (enabled) => {
    if (!enabled) {
      controller.invalidate();
      clearAutoFillNotice();
    }
  });

  watch(params.inputUrl, (value) => {
    if (value.trim() !== "") {
      controller.invalidate();
    }
  });

  onMounted(() => {
    if (isMyGo()) {
      isForeground.value = windowActive();
      offWindowActive = onWindowActive((active) => {
        isForeground.value = active;
        if (!active) {
          controller.invalidate();
        }
      });
      offSignal = events.clipboardSignal.on((signal) => {
        isForeground.value = true;
        triggerCheck("focus", signal);
      });
    }
    if (typeof window !== "undefined") {
      window.addEventListener("focus", onWindowFocus);
      window.addEventListener("blur", onWindowBlur);
    }
    if (typeof document !== "undefined") {
      document.addEventListener("visibilitychange", onVisibilityChange);
    }
    triggerCheck("startup");
  });

  onUnmounted(() => {
    controller.invalidate();
    clearAutoFillNotice();
    offSignal?.();
    offWindowActive?.();
    if (typeof window !== "undefined") {
      window.removeEventListener("focus", onWindowFocus);
      window.removeEventListener("blur", onWindowBlur);
    }
    if (typeof document !== "undefined") {
      document.removeEventListener("visibilitychange", onVisibilityChange);
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
