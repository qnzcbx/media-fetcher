<script setup lang="ts">
import { KeyRound } from "@lucide/vue";

defineProps<{
  onOpenCredentials: () => void;
}>();
</script>

<template>
  <header class="topbar">
    <div class="logo"></div>
    <div class="actions">
      <button @click="onOpenCredentials" class="icon-btn" title="凭证管理">
        <KeyRound class="icon" />
      </button>
    </div>
  </header>
</template>

<style scoped>
.topbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 24px;
  z-index: 10;
  -webkit-app-region: drag;
}

.logo {
  font-weight: 600;
  font-size: 18px;
  display: flex;
  align-items: center;
  gap: 8px;
}

.actions {
  -webkit-app-region: no-drag;
}

.icon-btn {
  background: none;
  border: none;
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--text-main);
  cursor: pointer;
  padding: 6px 12px;
  border-radius: 6px;
  transition: background 0.2s;
}

.icon-btn:hover {
  background: var(--input-bg);
}
</style>
