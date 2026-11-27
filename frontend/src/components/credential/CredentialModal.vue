<script setup lang="ts">
import { LogOut } from "@lucide/vue";
import BrandIcon from "../icons/BrandIcon.vue";
import BaseModal from "../common/BaseModal.vue";
import { computed } from "vue";
import type { SafeAccount } from "../../mygo";


const props = defineProps<{
  accounts: SafeAccount[];
  credentialError: string;
  busy?: boolean;
}>();

const account = computed(() => props.accounts.find((item) => item.siteId === "bilibili" && item.isActive) ?? props.accounts.find((item) => item.siteId === "bilibili"));

defineEmits<{
  (e: "close"): void;
  (e: "logout", account: SafeAccount): void;
  (e: "loginBilibili"): void;
}>();
</script>

<template>
  <BaseModal title="凭证管理" @close="$emit('close')">
    <p v-if="credentialError" role="alert">{{ credentialError }}</p>
    <div class="auth-card">
      <h4 class="auth-heading">
        <BrandIcon name="bilibili" :size="18" />
        <span>Bilibili</span>
      </h4>
      <div
        v-if="account"
        class="auth-status active"
      >
        <span>{{ account.name }} ({{ account.status === "active" ? "已登录" : "已过期" }})</span>
        <button class="logout-button" :disabled="busy" title="退出此账号" aria-label="退出此账号" @click="$emit('logout', account)"><LogOut :size="18" /></button>
      </div>
      <div v-else class="auth-action">
        <button
          class="btn-primary"
          style="width: 100%; justify-content: center"
          @click="$emit('loginBilibili')"
        >
          登录 Bilibili
        </button>
      </div>
    </div>

    <div class="auth-card">
      <h4 class="auth-heading">
        <BrandIcon name="lofter" :size="18" />
        <span>LOFTER</span>
      </h4>
      <div class="auth-status anonymous">匿名模式 (当前未配置凭证)</div>
    </div>
  </BaseModal>
</template>

<style scoped>
.auth-card {
  text-align: left;
  background: var(--input-bg);
  padding: 16px;
  border-radius: 8px;
  margin-bottom: 16px;
}
.auth-heading {
  display: grid;
  grid-template-columns: 18px 1fr;
  align-items: center;
  gap: 6px;
  line-height: 20px;
  margin: 0 0 12px 0;
  font-size: 15px;
}
.logout-button { border: none; background: transparent; color: var(--text-sub); padding: 6px; cursor: pointer; display: inline-flex; border-radius: 6px; }
.logout-button:hover:not(:disabled) { color: var(--danger); }
.logout-button:disabled { opacity: .4; cursor: not-allowed; }
.logout-button:focus-visible { outline: 2px solid var(--focus-ring); }
.auth-status {
  font-size: 14px;
  color: var(--text-sub);
}
.auth-status.active {
 display: flex; align-items: center; justify-content: space-between; gap: 8px;
  color: var(--selection-text);
  font-weight: 500;
}
</style>
