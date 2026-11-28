import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(__dirname, "..");

const utilSrc = readFileSync(path.join(root, "src/utils/runtimeStatus.ts"), "utf8");
const sectionSrc = readFileSync(path.join(root, "src/components/settings/RuntimeStatusSection.vue"), "utf8");
const drawerSrc = readFileSync(path.join(root, "src/components/settings/SettingsDrawer.vue"), "utf8");

// 1. 来源映射覆盖：内置 / 系统 / 用户指定
assert.match(utilSrc, /case "bundled":[\s\S]*return "内置"/, "bundled must map to 内置");
assert.match(utilSrc, /case "system":[\s\S]*return "系统"/, "system must map to 系统");
assert.match(utilSrc, /case "explicit":[\s\S]*return "用户指定"/, "explicit must map to 用户指定");

// 2. 设置页仅按需展开详细诊断路径，不将绝对路径/DLL/哈希直接平铺在主设置列表中
assert.match(drawerSrc, /<RuntimeStatusSection :open="open" \/>/, "SettingsDrawer must mount RuntimeStatusSection");
assert.match(sectionSrc, /DesktopService\.getRuntimeStatus\(\)/, "RuntimeStatusSection must call DesktopService.getRuntimeStatus");
assert.match(sectionSrc, /const showDiagnostics = ref\(false\)/, "Diagnostics must default to collapsed");
assert.match(sectionSrc, /v-if="showDiagnostics"/, "Detailed paths must only render inside expandable diagnostics");

// 3. 确保本阶段不包含在线安装/更新/回滚按钮
assert.doesNotMatch(sectionSrc, /自动更新|立即更新|在线安装|回滚/, "Settings UI must not include auto-update/install/rollback controls in 03.7C.2");

console.log("runtime-status contract tests passed.");
