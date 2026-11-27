import { ref, onUnmounted } from "vue";
import { getAccounts, logoutAccount, generateBiliQRCode, pollBiliQRCode } from "../api/credential";
import type { MediaInfoResponse, SafeAccount } from "../mygo";
import { extractSupportedURL, getSiteIdFromURL } from "../utils/url";

export const isValidAccount = (account: SafeAccount) => account.status === "active";
export const findActiveAccount = (accounts: SafeAccount[], siteId: string) =>
  accounts.find((account) => account.siteId === siteId && account.isActive && isValidAccount(account));

export function createCredentialState(query: typeof getAccounts, remove = logoutAccount) {
  const accounts = ref<SafeAccount[]>([]);
  const isInitialized = ref(false);
  const credentialError = ref("");
  let pending: Promise<boolean> | undefined;

  const fetchAccounts = () => pending ??= Promise.resolve().then(query).then((result) => {
    accounts.value = result ?? [];
    isInitialized.value = true;
    credentialError.value = "";
    return true;
  }).catch(() => {
    isInitialized.value = false;
    credentialError.value = "无法读取凭证，请重试。";
    return false;
  }).finally(() => { pending = undefined; });

  const logout = async (account: SafeAccount) => {
    if (pending) await pending;
    try {
      await remove(account.siteId, account.accountId);
    } catch (error) {
      // 删除失败也可能已撤销内存会话，刷新后仍明确返回失败。
      await fetchAccounts();
      throw error;
    }
    accounts.value = accounts.value.filter((item) => item.siteId !== account.siteId || item.accountId !== account.accountId);
    const refreshed = await fetchAccounts();
    if (!refreshed) throw new Error("本地凭证已退出，但账号列表刷新失败，请重新打开凭证管理。");
  };

  const ensureInitialized = () => pending ?? (isInitialized.value ? Promise.resolve(true) : fetchAccounts());
  const getActiveAccountId = (siteId: string) =>
    isInitialized.value ? findActiveAccount(accounts.value, siteId)?.accountId ?? "" : "";

  return { logout, accounts, isInitialized, credentialError, fetchAccounts, ensureInitialized, getActiveAccountId };
}

export function resolveParseTarget(input: string, getAccountId: (site: string) => string) {
  const url = extractSupportedURL(input);
  const siteId = getSiteIdFromURL(url);
  return { url, siteId, accountId: siteId ? getAccountId(siteId) : "" };
}

export const canUnlock = (media: MediaInfoResponse | null, initialized: boolean, accountId: string) =>
  media?.siteId === "bilibili" && media.isAnonymous && initialized && !accountId;

const credentials = createCredentialState(getAccounts);

export const useCredentials = () => {
  const showAuthManage = ref(false);
  const showBiliQRModal = ref(false);
  const qrUrl = ref("");
  const qrStatus = ref("");
  let pollTimer: ReturnType<typeof setTimeout> | undefined;
  let session = 0;

  const stopPolling = () => {
    session++;
    clearTimeout(pollTimer);
  };
  const closeQRModal = () => {
    stopPolling();
    showBiliQRModal.value = false;
  };

  const loginBilibili = async (onSuccess?: (accountId: string, isCurrent: () => boolean) => Promise<void>) => {
    stopPolling();
    const id = session;
    const isCurrent = () => id === session;
    showAuthManage.value = false;
    showBiliQRModal.value = true;
    qrStatus.value = "申请二维码中...";
    qrUrl.value = "";

    const poll = async (key: string) => {
      try {
        const res = await pollBiliQRCode(key);
        if (!isCurrent()) return;
        qrStatus.value = res.message;
        if (res.state === "confirmed") {
          // 先等旧查询结束，再读取扫码成功后的凭证。
          await credentials.ensureInitialized();
          if (!isCurrent()) return;
          const refreshed = await credentials.fetchAccounts();
          if (!isCurrent()) return;
          const accountId = credentials.getActiveAccountId("bilibili");
          if (!refreshed || !accountId) {
            qrStatus.value = credentials.credentialError.value || "未找到有效的 Bilibili 活动账号";
            stopPolling();
            return;
          }
          await onSuccess?.(accountId, isCurrent);
          if (isCurrent()) closeQRModal();
        } else if (res.state === "expired" || res.state === "failed" || res.state === "cancelled") {
          stopPolling();
        } else {
          pollTimer = setTimeout(() => void poll(key), 3000);
        }
      } catch {
        if (!isCurrent()) return;
        qrStatus.value = "扫码状态查询失败，请重新登录。";
        stopPolling();
      }
    };

    try {
      const res = await generateBiliQRCode();
      if (!isCurrent()) return;
      qrUrl.value = res.url;
      qrStatus.value = "请使用手机 Bilibili App 扫描二维码";
      void poll(res.qrcodeKey);
    } catch {
      if (!isCurrent()) return;
      qrStatus.value = "二维码申请失败，请重试。";
      stopPolling();
    }
  };

  void credentials.ensureInitialized();
  onUnmounted(stopPolling);

  return { ...credentials, showAuthManage, showBiliQRModal, qrUrl, qrStatus, loginBilibili, closeQRModal };
};
