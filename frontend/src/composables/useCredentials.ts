import { ref, onUnmounted } from "vue";
import { getAccounts, generateBiliQRCode, pollBiliQRCode } from "../api/credential";

export type SafeAccount = {
  siteId: string;
  accountId: string;
  name: string;
  avatarUrl: string;
  status: string;
  isActive: boolean;
};

// 单例共享状态
const accounts = ref<SafeAccount[]>([]);
let isInitialized = false;

export const useCredentials = () => {
  const showAuthManage = ref(false);
  const showBiliQRModal = ref(false);
  const qrUrl = ref("");
  const qrStatus = ref("");
  let pollTimer: ReturnType<typeof setTimeout> | null = null;
  let isPolling = false;

  const fetchAccounts = async () => {
    try {
      accounts.value = (await getAccounts()) || [];
      isInitialized = true;
    } catch (e) {
      console.error(e);
      // 查询失败不能伪装成已登录或改变初始化状态以防误判，但为了流程不卡死，也可以标记为 true 或在 UI 中显示错误
      isInitialized = true;
    }
  };

  // 保证只有第一次被调用时才会执行
  const ensureInitialized = async () => {
    if (!isInitialized) {
      await fetchAccounts();
    }
  };

  // 立即在模块加载或组件使用时触发一次
  ensureInitialized();

  const getActiveAccountId = (siteId: string): string => {
    const active = accounts.value.find(
      (a) => a.isActive && a.siteId === siteId && a.status === "Valid"
    );
    return active ? active.accountId : "";
  };

  const stopPolling = () => {
    isPolling = false;
    if (pollTimer) {
      clearTimeout(pollTimer);
      pollTimer = null;
    }
  };

  const pollQRCode = async (key: string) => {
    if (!isPolling || !showBiliQRModal.value) return;
    try {
      const res = await pollBiliQRCode(key);
      // 如果已经停止轮询，则忽略延迟的旧请求结果
      if (!isPolling) return;

      qrStatus.value = res.message;
      if (res.state === "confirmed") {
        await fetchAccounts();
        stopPolling();
        if (loginSuccessCallback) {
          loginSuccessCallback();
          loginSuccessCallback = null;
        }
        setTimeout(() => {
          showBiliQRModal.value = false;
        }, 1500);
      } else if (res.state === "expired" || res.state === "failed") {
        stopPolling();
      } else {
        pollTimer = setTimeout(() => pollQRCode(key), 3000);
      }
    } catch (e) {
      if (!isPolling) return;
      qrStatus.value = "轮询错误";
      pollTimer = setTimeout(() => pollQRCode(key), 3000);
    }
  };

  let loginSuccessCallback: (() => void) | null = null;

  const loginBilibili = async (onSuccess?: () => void) => {
    showAuthManage.value = false;
    showBiliQRModal.value = true;
    qrStatus.value = "申请二维码中...";
    qrUrl.value = "";
    isPolling = true;
    if (pollTimer) clearTimeout(pollTimer);
    loginSuccessCallback = onSuccess || null;

    try {
      const res = await generateBiliQRCode();
      if (!isPolling) return;
      qrUrl.value = res.url;
      qrStatus.value = "请使用手机 Bilibili App 扫描二维码";
      pollQRCode(res.qrcodeKey);
    } catch (e: any) {
      if (!isPolling) return;
      qrStatus.value = "错误: " + e.message;
      isPolling = false;
    }
  };

  const closeQRModal = () => {
    showBiliQRModal.value = false;
    stopPolling();
    loginSuccessCallback = null;
  };

  onUnmounted(() => {
    stopPolling();
  });

  return {
    accounts,
    showAuthManage,
    showBiliQRModal,
    qrUrl,
    qrStatus,
    fetchAccounts,
    ensureInitialized,
    getActiveAccountId,
    loginBilibili,
    closeQRModal,
    stopPolling,
  };
};
