<script setup lang="ts">
import BaseModal from "../common/BaseModal.vue";
import QrcodeVue from "qrcode.vue";

defineProps<{
  qrUrl: string;
  qrStatus: string;
}>();

defineEmits<{
  (e: "close"): void;
}>();
</script>

<template>
  <BaseModal title="登录 Bilibili" @close="$emit('close')">
    <div class="qr-container">
      <qrcode-vue v-if="qrUrl" :value="qrUrl" :size="200" level="M" render-as="svg" />
      <div v-else class="qr-placeholder">加载中...</div>
    </div>
    <p class="qr-status">{{ qrStatus }}</p>
  </BaseModal>
</template>

<style scoped>
.qr-container {
  width: 200px;
  height: 200px;
  margin: 0 auto 16px;
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 8px;
}

.qr-container img {
  width: 100%;
  height: 100%;
}

.qr-placeholder {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-sub);
  background: var(--input-bg);
}

.qr-status {
  color: var(--text-sub);
  font-size: 14px;
  margin-bottom: 0;
}
</style>
