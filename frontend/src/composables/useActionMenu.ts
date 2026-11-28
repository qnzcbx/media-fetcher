import { computed, onBeforeUnmount, ref, watch, type Ref } from "vue";

// 全局唯一当前展开的更多操作菜单标识，确保同一时间最多只有一个菜单展开。
const activeMenuId = ref<symbol | null>(null);

export const closeActiveActionMenu = () => {
  activeMenuId.value = null;
};

export type ActionMenuRefs = {
  menuRef: Ref<HTMLElement | null>;
  triggerRef: Ref<HTMLElement | null>;
  extraTriggerRef?: Ref<HTMLElement | null>;
  contextRootRef?: Ref<HTMLElement | null>;
};

const containsTarget = (el: HTMLElement | null | undefined, target: EventTarget | null) =>
  !!el && target instanceof Node && el.contains(target);

// 统一管理下载卡片与下载列表项的「更多操作」浮层：
// 支持互斥展开、外部点击关闭（不拦截目标正常交互）、Escape 关闭、滚动列表关闭与卸载清理。
export const useActionMenu = (refs: ActionMenuRefs) => {
  const id = Symbol("action-menu");
  const isOpen = computed(() => activeMenuId.value === id);
  let attached = false;

  const isInsideOwnControls = (target: EventTarget | null) =>
    containsTarget(refs.menuRef.value, target) ||
    containsTarget(refs.triggerRef.value, target) ||
    containsTarget(refs.extraTriggerRef?.value, target);

  const close = () => {
    if (activeMenuId.value === id) {
      activeMenuId.value = null;
    }
  };

  const open = () => {
    activeMenuId.value = id;
  };

  const toggle = () => {
    activeMenuId.value = activeMenuId.value === id ? null : id;
  };

  const onPointerDown = (event: PointerEvent) => {
    // 右键交由对应元素的 contextmenu 处理，左键/中键点击外部立即关闭且不阻止后续点击行为。
    if (event.button === 2) return;
    if (!isInsideOwnControls(event.target)) {
      close();
    }
  };

  const onContextMenu = (event: MouseEvent) => {
    if (!isInsideOwnControls(event.target) && !containsTarget(refs.contextRootRef?.value, event.target)) {
      close();
    }
  };

  const onKeyDown = (event: KeyboardEvent) => {
    if (event.key === "Escape") {
      close();
    }
  };

  const onScroll = (event: Event) => {
    // 滚动发生在菜单外部容器（如列表、页面）时关闭浮层。
    if (!containsTarget(refs.menuRef.value, event.target)) {
      close();
    }
  };

  const attach = () => {
    if (attached || typeof document === "undefined" || typeof window === "undefined") return;
    attached = true;
    document.addEventListener("pointerdown", onPointerDown, true);
    document.addEventListener("contextmenu", onContextMenu, true);
    document.addEventListener("keydown", onKeyDown);
    window.addEventListener("scroll", onScroll, true);
  };

  const detach = () => {
    if (!attached || typeof document === "undefined" || typeof window === "undefined") return;
    attached = false;
    document.removeEventListener("pointerdown", onPointerDown, true);
    document.removeEventListener("contextmenu", onContextMenu, true);
    document.removeEventListener("keydown", onKeyDown);
    window.removeEventListener("scroll", onScroll, true);
  };

  watch(
    isOpen,
    (openNow) => {
      if (openNow) attach();
      else detach();
    },
    { immediate: true },
  );

  onBeforeUnmount(() => {
    detach();
    close();
  });

  return {
    isOpen,
    open,
    close,
    toggle,
  };
};
