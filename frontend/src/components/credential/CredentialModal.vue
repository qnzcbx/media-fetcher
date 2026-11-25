<script setup lang="ts">
import BrandIcon from "../icons/BrandIcon.vue";
import BaseModal from "../common/BaseModal.vue";
import { computed } from "vue";
import type { SafeAccount } from "../../mygo";
import { findActiveAccount } from "../../composables/useCredentials";

const props = defineProps<{
  accounts: SafeAccount[];
  credentialError: string;
}>();

const account = computed(() => props.credentialError ? undefined : findActiveAccount(props.accounts, "bilibili"));

defineEmits<{
  (e: "close"): void;
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
        {{ account.name }} (已登录)
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
.auth-status {
  font-size: 14px;
  color: var(--text-sub);
}
.auth-status.active {
  color: var(--selection-text);
  font-weight: 500;
}
</style>
