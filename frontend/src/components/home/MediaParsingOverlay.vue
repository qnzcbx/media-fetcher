<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from "vue";

const SITE_NAMES: Record<string, string> = {
  youtube: "YouTube",
  bilibili: "Bilibili",
  lofter: "LOFTER",
};

const props = withDefaults(
  defineProps<{
    active: boolean;
    siteId?: string;
    delayMs?: number;
  }>(),
  {
    siteId: "",
    delayMs: 500,
  },
);

const visible = ref(false);
let timer: ReturnType<typeof setTimeout> | null = null;

const clearTimer = () => {
  if (timer !== null) {
    clearTimeout(timer);
    timer = null;
  }
};

watch(
  () => props.active,
  (isActive) => {
    clearTimer();
    if (!isActive) {
      visible.value = false;
      return;
    }
    if (props.delayMs <= 0) {
      visible.value = true;
      return;
    }
    timer = setTimeout(() => {
      timer = null;
      if (props.active) visible.value = true;
    }, props.delayMs);
  },
  { immediate: true },
);

onBeforeUnmount(() => {
  clearTimer();
  visible.value = false;
});

const subtitle = computed(() => {
  const label = SITE_NAMES[(props.siteId ?? "").toLowerCase()];
  return label ? `正在获取 ${label} 的可用媒体资源` : "正在获取可用媒体资源";
});
</script>

<template>
  <Transition name="parsing-fade">
    <div
      v-if="visible"
      class="parsing-overlay"
      role="status"
      aria-live="polite"
      aria-busy="true"
    >
      <div class="parsing-indicator">
        <span class="parsing-spinner" aria-hidden="true"></span>
        <p class="parsing-title">正在全力解析</p>
        <p class="parsing-subtitle">{{ subtitle }}</p>
      </div>
    </div>
  </Transition>
</template>

<style scoped>
.parsing-overlay {
  position: fixed;
  inset: 0;
  z-index: 90;
  display: grid;
  place-items: center;
  background: var(--parsing-overlay-bg);
  backdrop-filter: blur(2px);
  -webkit-backdrop-filter: blur(2px);
}

.parsing-indicator {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  text-align: center;
  padding: 0 24px;
  user-select: none;
}

.parsing-spinner {
  width: 38px;
  height: 38px;
  margin-bottom: 4px;
  border-radius: 50%;
  border: 3px solid var(--parsing-spinner-track);
  border-top-color: var(--parsing-spinner-head);
  border-right-color: var(--parsing-spinner-head);
  animation: parsing-spin 0.95s linear infinite;
}

.parsing-title {
  margin: 0;
  font-size: 18px;
  font-weight: 500;
  letter-spacing: 0.02em;
  color: var(--parsing-text);
  text-shadow: 0 1px 10px rgba(0, 0, 0, 0.24);
}

.parsing-subtitle {
  margin: 0;
  font-size: 13px;
  color: var(--parsing-subtext);
  text-shadow: 0 1px 8px rgba(0, 0, 0, 0.2);
}

@keyframes parsing-spin {
  to {
    transform: rotate(360deg);
  }
}

@media (prefers-reduced-motion: reduce) {
  .parsing-spinner {
    animation: none;
  }
}

.parsing-fade-enter-active,
.parsing-fade-leave-active {
  transition: opacity 0.2s ease;
}

.parsing-fade-enter-from,
.parsing-fade-leave-to {
  opacity: 0;
}
</style>
