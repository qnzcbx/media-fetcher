<script setup lang="ts">
import { DialogRoot, DialogPortal, DialogOverlay, DialogContent, DialogTitle, DialogDescription } from "reka-ui";
defineProps<{ confirmation?: { ids: string[]; names: string[]; skipped: number } }>();
const emit = defineEmits<{ cancel: []; confirm: [] }>();
</script>
<template>
  <DialogRoot :open="!!confirmation" @update:open="(open) => !open && emit('cancel')">
    <DialogPortal>
      <DialogOverlay class="recycle-overlay" />
      <DialogContent class="recycle-dialog">
        <DialogTitle>删除文件</DialogTitle>
        <p v-if="confirmation?.names.length === 1" class="filename">{{ confirmation.names[0] }}</p>
        <DialogDescription>将 {{ confirmation?.ids.length }} 个文件移至回收站，并移除对应记录？</DialogDescription>
        <p v-if="confirmation?.skipped">跳过 {{ confirmation.skipped }} 项不可删除的记录。</p>
        <div class="buttons"><button class="btn-cancel" @click="emit('cancel')">取消</button><button class="delete" @click="emit('confirm')">删除</button></div>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>
<style scoped>
.recycle-overlay { position:fixed; inset:0; background:var(--overlay-bg); z-index:200; }
.recycle-dialog { position:fixed; left:50%; top:50%; transform:translate(-50%,-50%); background:var(--card-bg); color:var(--text-main); border-radius:14px; padding:24px; width:min(360px,calc(100vw - 64px)); z-index:201; }
.filename { overflow-wrap:anywhere; }
.buttons { display:flex; justify-content:flex-end; gap:12px; margin-top:22px; }
.delete { border:0; border-radius:6px; padding:8px 16px; background:var(--danger); color:white; cursor:pointer; }
</style>
