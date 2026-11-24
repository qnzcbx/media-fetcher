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

export const useCredentials = () => {
  const accounts = ref<SafeAccount[]>([]);
  const showAuthManage = ref(false);
  const showBiliQRModal = ref(false);
  const qrUrl = ref("");
  const qrStatus = ref("");
  let pollTimer: ReturnType<typeof setTimeout> | null = null;
  let isPolling = false;

  const fetchAccounts = async () => {
    try {
      accounts.value = (await getAccounts()) || [];
    } catch (e) {
      console.error(e);
    }
  };

  const getActiveAccountId = (siteId: string): string => {
    const active = accounts.value.find((a) => a.isActive && a.siteId === siteId);
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
      qrStatus.value = res.message;
      if (res.state === "confirmed") {
        await fetchAccounts();
        stopPolling();
        setTimeout(() => {
          showBiliQRModal.value = false;
        }, 1500);
      } else if (res.state === "expired" || res.state === "failed") {
        stopPolling();
      } else {
        pollTimer = setTimeout(() => pollQRCode(key), 3000);
      }
    } catch (e) {
      qrStatus.value = "轮询错误";
      pollTimer = setTimeout(() => pollQRCode(key), 3000);
    }
  };

  const loginBilibili = async () => {
    showAuthManage.value = false;
    showBiliQRModal.value = true;
    qrStatus.value = "申请二维码中...";
    qrUrl.value = "";
    isPolling = true;

    try {
      const res = await generateBiliQRCode();
      qrUrl.value = res.url;
      qrStatus.value = "请使用手机 Bilibili App 扫描二维码";
      pollQRCode(res.qrcodeKey);
    } catch (e: any) {
      qrStatus.value = "错误: " + e.message;
      isPolling = false;
    }
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
    getActiveAccountId,
    loginBilibili,
    stopPolling,
  };
};
