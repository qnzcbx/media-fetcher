<script setup lang="ts">
import {
  DialogRoot,
  DialogPortal,
  DialogOverlay,
  DialogContent,
  DialogTitle,
  DialogDescription,
} from 'reka-ui'

const props = withDefaults(defineProps<{
  open: boolean;
  side?: "left" | "right" | "top" | "bottom";
  width?: string;
  duration?: number;
  overlayOpacity?: number;
  title?: string;
  description?: string;
}>(), {
  side: "right",
  width: "500px",
  duration: 220,
  overlayOpacity: 0.25,
});

const emit = defineEmits<{
  (e: 'update:open', value: boolean): void;
}>();
</script>

<template>
  <DialogRoot :open="open" @update:open="(val: boolean) => emit('update:open', val)">
    <DialogPortal>
      <Transition name="sheet-overlay">
        <DialogOverlay
          v-if="open"
          force-mount
          class="sheet-overlay"
          :style="{
            '--overlay-opacity': overlayOpacity,
            '--duration': `${duration}ms`
          }"
        />
      </Transition>
      <Transition :name="`sheet-${side}`">
        <DialogContent
          v-if="open"
          force-mount
          class="sheet-content"
          :class="[`side-${side}`]"
          :style="{
            '--sheet-width': width,
            '--duration': `${duration}ms`
          }"
        >
          <DialogTitle v-if="title" class="sr-only">{{ title }}</DialogTitle>
          <DialogDescription v-if="description" class="sr-only">{{ description }}</DialogDescription>
          <slot></slot>
        </DialogContent>
      </Transition>
    </DialogPortal>
  </DialogRoot>
</template>

<style>
.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border-width: 0;
}

.sheet-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, var(--overlay-opacity));
  z-index: 999;
}

.sheet-content {
  position: fixed;
  z-index: 1000;
  background: var(--bg-color);
  box-shadow: 0 0 24px var(--shadow-color);
  display: flex;
  flex-direction: column;
}
.side-right {
  top: 0;
  bottom: 0;
  right: 0;
  height: 100dvh;
  width: min(100%, var(--sheet-width));
}

/* Transitions */
.sheet-overlay-enter-active,
.sheet-overlay-leave-active {
  transition: opacity var(--duration) ease;
}
.sheet-overlay-enter-from,
.sheet-overlay-leave-to {
  opacity: 0;
}

.sheet-right-enter-active,
.sheet-right-leave-active {
  transition: transform var(--duration) cubic-bezier(0.4, 0, 0.2, 1);
}
.sheet-right-enter-from,
.sheet-right-leave-to {
  transform: translateX(100%);
}

@media (prefers-reduced-motion: reduce) {
  .sheet-overlay-enter-active,
  .sheet-overlay-leave-active,
  .sheet-right-enter-active,
  .sheet-right-leave-active {
    transition: none !important;
  }
}
</style>
