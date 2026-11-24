<script setup lang="ts">
defineProps<{
  title: string;
}>();

defineEmits<{
  (e: "close"): void;
}>();
</script>

<template>
  <div class="modal-overlay">
    <div class="modal auth-modal">
      <h3>{{ title }}</h3>
      <slot></slot>
      <button class="btn-cancel" style="margin-top: 24px; width: 100%" @click="$emit('close')">
        关闭
      </button>
    </div>
  </div>
</template>

<style scoped>
.auth-modal {
  background: var(--card-bg);
  padding: 32px;
  border-radius: 16px;
  width: 320px;
  text-align: center;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.1);
}

h3 {
  margin: 0 0 24px 0;
}
</style>
