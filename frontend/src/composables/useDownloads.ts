import { ref, onMounted, onUnmounted } from "vue";
import { startDownload as apiStartDownload, cancelDownload as apiCancelDownload, onProgress, offProgress } from "../api/download";

// 下载状态单例，保证全局只有一个下载状态，避免重复或丢失
const isDownloading = ref(false);
const progressInfo = ref<any>(null);

export const useDownloads = () => {
  const handleProgress = (info: any) => {
    progressInfo.value = info;
    if (info.stage === "done" || info.stage === "error") {
      setTimeout(() => {
        isDownloading.value = false;
      }, 5000);
    }
  };

  // 因为需要全局单例监听，可以仅在一个组件或顶层挂载，这里使用安全的监听和移除机制
  onMounted(() => {
    onProgress(handleProgress);
  });

  onUnmounted(() => {
    offProgress(handleProgress);
  });

  const doDownload = async (params: any) => {
    isDownloading.value = true;
    progressInfo.value = { stage: "starting", message: "准备下载..." };
    try {
      await apiStartDownload(params);
    } catch (e: any) {
      console.error(e);
      progressInfo.value = { stage: "error", message: e.message };
      // 启动失败时状态不要重置，以便给错误提示
    }
  };

  const doCancel = async () => {
    await apiCancelDownload();
    isDownloading.value = false;
    progressInfo.value = null;
  };

  return {
    isDownloading,
    progressInfo,
    doDownload,
    doCancel,
  };
};
