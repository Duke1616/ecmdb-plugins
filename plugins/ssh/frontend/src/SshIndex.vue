<template>
  <div class="term-container">
    <!-- 连接选择对话框 (恢复使用全局注册的 FormDialog，保证 UI 样式与主前端 100% 相同) -->
    <FormDialog
      v-model="dialogVisible"
      :title="title || '选择连接方式'"
      :subtitle="`请选择连接到资源 ${resourceId} 的方式`"
      :width="600"
      :show-close="false"
      :close-on-click-modal="false"
      :confirm-text="loading ? '连接中...' : '连接'"
      :confirm-loading="loading"
      :confirm-disabled="!selectedOption || loading"
      :footer-info-text="selectedOption ? `已选择: ${getCurrentOptionLabel()}` : '请选择一种连接方式'"
      header-icon="Connection"
      @cancel="handleCancel"
      @confirm="connect"
    >
      <div class="connection-options">
        <div
          v-for="option in connectionOptions"
          :key="option.value"
          class="connection-option"
          :class="{
            selected: selectedOption === option.value,
            disabled: option.disabled
          }"
          @click="selectOption(option)"
        >
          <div class="option-icon">
            <el-icon :size="24">
              <component :is="option.icon" />
            </el-icon>
          </div>
          <div class="option-content">
            <div class="option-title">{{ option.label }}</div>
            <div class="option-description">{{ option.description }}</div>
          </div>
          <div v-if="option.disabled" class="option-badge">
            <el-tag type="warning" size="small">暂不可用</el-tag>
          </div>
        </div>
      </div>
    </FormDialog>

    <!-- 连接状态提示 -->
    <div v-if="isConnected" class="connection-status">
      <div class="status-chip">
        <div class="status-chip__dot" />
        <span>已连接到 {{ getCurrentOptionLabel() }}</span>
      </div>
      <div class="status-meta">资源 ID {{ resourceId }}</div>
      <el-button link type="danger" @click="disconnect" class="disconnect-btn">断开连接</el-button>
    </div>

    <div class="terminal-panel">
      <div v-if="!showTerminalView" class="terminal-empty-state">
        <el-empty :description="hasResource ? '当前未建立连接' : '请先从左侧选择主机'">
          <el-button v-if="hasResource" type="primary" @click="reopenDialog">选择连接方式</el-button>
        </el-empty>
      </div>

      <!-- 终端组件容器 -->
      <div v-else class="terminal-wrapper">
        <finder
          v-if="selectedOption === 'Web Sftp'"
          :key="finderViewKey"
          :resource_id="resourceId"
          :prefix="prefix"
          :api-base="apiBase"
        />
        <xterm
          v-else-if="selectedOption === 'Web Shell'"
          :key="xtermViewKey"
          :resource_id="resourceId"
          :prefix="prefix"
          :api-base="apiBase"
        />
        <guacd v-else-if="selectedOption === 'RDP'" :key="guacdViewKey" :resource_id="resourceId" :prefix="prefix" />
        <!-- VNC 组件暂未实现 -->
        <div v-else-if="selectedOption === 'VNC'" class="vnc-placeholder">
          <el-empty description="VNC 功能暂未实现" />
        </div>
      </div>
    </div>
  </div>
</template>

<script lang="ts" setup>
import { ref, computed, onMounted, onUnmounted, watch } from "vue"
import { ElMessage } from "element-plus"

// 引入插件内部业务视图组件
import guacd from "./guacd.vue"
import xterm from "./xterm.vue"
import finder from "./file-system.vue"

// 引入本地地址配置
import type { PrefixConfig } from "./utils/prefix-config"
import { getPrefixConfig } from "./utils/prefix-config"
import { getRuntimeRequestHeaders } from "./utils/runtime-auth"

// Props
interface SshIndexProps {
  apiBase: string
  resourceId: string
  connectionType?: string
  title?: string
}

const props = defineProps<SshIndexProps>()

interface ConnectionOption {
  value: string
  label: string
  description: string
  icon: string
  disabled?: boolean
}

// 计算属性
const hasResource = computed(() => Boolean(props.resourceId))
const sessionVersion = ref(0)
const showTerminalView = computed(
  () => hasResource.value && isConnected.value && Boolean(selectedOption.value) && Boolean(prefix.value?.wsServer)
)
const terminalViewKey = computed(() => `${sessionVersion.value}:${selectedOption.value}:${props.resourceId || "empty"}`)
const finderViewKey = computed(() => `finder:${terminalViewKey.value}`)
const xtermViewKey = computed(() => `xterm:${terminalViewKey.value}`)
const guacdViewKey = computed(() => `guacd:${terminalViewKey.value}`)

// 状态管理
const dialogVisible = ref<boolean>(true)
const isConnected = ref<boolean>(false)
const loading = ref<boolean>(false)
const selectedOption = ref<string>("")
const prefix = ref<PrefixConfig>()

// 连接配置选项
const connectionOptions: ConnectionOption[] = [
  {
    value: "Web Shell",
    label: "Web Shell",
    description: "基于 Web 的终端连接",
    icon: "Stamp"
  },
  {
    value: "Web Sftp",
    label: "文件管理器",
    description: "SFTP 文件传输和管理",
    icon: "FolderOpened"
  },
  {
    value: "RDP",
    label: "远程桌面",
    description: "Windows 远程桌面连接",
    icon: "Monitor"
  },
  {
    value: "VNC",
    label: "VNC 连接",
    description: "VNC 远程桌面连接",
    icon: "VideoCamera",
    disabled: true
  }
]

const getCurrentOptionLabel = () => {
  const option = connectionOptions.find((opt) => opt.value === selectedOption.value)
  return option?.label || selectedOption.value
}

const getPreferredOption = () => {
  if (props.connectionType) {
    const option = connectionOptions.find((opt) => opt.value === props.connectionType && !opt.disabled)
    if (option) {
      return option
    }
  }
  return connectionOptions.find((opt) => !opt.disabled)
}

const preselectConnectionOption = () => {
  selectedOption.value = getPreferredOption()?.value || ""
}

// 方法
const selectOption = (option: ConnectionOption) => {
  if (option.disabled) {
    ElMessage.warning(`${option.label} 功能暂不可用`)
    return
  }
  selectedOption.value = option.value
}

const handleCancel = () => {
  dialogVisible.value = false
}

const reopenDialog = () => {
  dialogVisible.value = true
}

const connect = async () => {
  if (!selectedOption.value) {
    ElMessage.warning("请选择连接方式")
    return
  }

  if (!props.resourceId) {
    ElMessage.error("缺少资源ID参数")
    return
  }

  loading.value = true

  try {
    const response = await fetch(`${props.apiBase}/terminal/connect`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        ...getRuntimeRequestHeaders()
      },
      credentials: "include",
      body: JSON.stringify({
        resource_id: Number(props.resourceId),
        type: selectedOption.value
      })
    })

    if (!response.ok) {
      throw new Error(`建立连接失败: HTTP ${response.status}`)
    }

    prefix.value = getPrefixConfig()
    sessionVersion.value += 1
    isConnected.value = true
    dialogVisible.value = false

    ElMessage.success(`成功连接到 ${getCurrentOptionLabel()}`)
  } catch (error: any) {
    console.error("连接失败:", error)
    ElMessage.error(error.message || "建立连接发生错误")
  } finally {
    loading.value = false
  }
}

const disconnect = () => {
  isConnected.value = false
  prefix.value = undefined
  preselectConnectionOption()
  dialogVisible.value = true

  ElMessage.info("已断开连接")
}

const resetConnectionState = () => {
  isConnected.value = false
  prefix.value = undefined
  preselectConnectionOption()
  dialogVisible.value = Boolean(props.resourceId)
}

// 页面离开前确认
const handleBeforeUnload = (event: BeforeUnloadEvent) => {
  if (isConnected.value) {
    event.preventDefault()
    event.returnValue = "您确定要离开吗？连接可能会断开。"
    return event.returnValue
  }
}

// 动态管理 beforeunload 事件监听器
watch(
  () => isConnected.value,
  (connected) => {
    if (connected) {
      window.addEventListener("beforeunload", handleBeforeUnload)
    } else {
      window.removeEventListener("beforeunload", handleBeforeUnload)
    }
  },
  { immediate: true }
)

watch(
  () => props.resourceId,
  (next, prev) => {
    if (!next || !prev || next === prev) return
    resetConnectionState()
  }
)

watch(
  () => props.connectionType,
  () => {
    if (!props.resourceId) return
    resetConnectionState()
  }
)

onMounted(() => {
  if (!props.resourceId) {
    dialogVisible.value = false
    return
  }

  preselectConnectionOption()
  dialogVisible.value = true
})

onUnmounted(() => {
  window.removeEventListener("beforeunload", handleBeforeUnload)
})
</script>

<style scoped lang="scss">
.term-container {
  display: flex;
  flex-direction: column;
  flex: 1;
  height: 100%;
  min-height: 0;
  min-width: 0;
  gap: 12px;
  box-sizing: border-box;
  background: transparent;
  width: 100%;
  overflow: hidden;
}

// 恢复原有 100% 样式配置，保证视觉细节毫无差别
.connection-options {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 20px;
  padding: 20px 0;
}

.connection-option {
  display: flex;
  align-items: center;
  padding: 20px;
  border-radius: 8px;
  border: 1px solid #dbe3ee;
  background: white;
  cursor: pointer;
  box-shadow: 0 8px 20px rgba(15, 23, 42, 0.05);
  transition:
    border-color 0.2s ease,
    box-shadow 0.2s ease,
    transform 0.2s ease;
  position: relative;
  overflow: hidden;

  &::before {
    content: "";
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    height: 2px;
    background: #409eff;
    transform: scaleX(0);
    transition: transform 0.3s ease;
  }

  &:hover {
    border-color: #bfd7f5;
    transform: translateY(-1px);
    box-shadow: 0 12px 24px rgba(15, 23, 42, 0.08);

    &::before {
      transform: scaleX(1);
    }
  }

  &.selected {
    border-color: #409eff;
    background: #f7fbff;
    color: #0f172a;
    box-shadow: 0 12px 24px rgba(64, 158, 255, 0.12);

    &::before {
      background: #409eff;
      transform: scaleX(1);
    }

    .option-title {
      color: #0f172a;
    }

    .option-description {
      color: #475569;
    }
  }

  &.disabled {
    opacity: 0.6;
    cursor: not-allowed;
    background: #f5f7fa;

    &:hover {
      transform: none;
      box-shadow: 0 8px 20px rgba(15, 23, 42, 0.05);
      border-color: #dbe3ee;

      &::before {
        transform: scaleX(0);
      }
    }
  }
}

.option-icon {
  margin-right: 16px;
  color: #409eff;
  transition: color 0.3s ease;
}

.connection-option.selected .option-icon {
  color: #409eff;
}

.option-content {
  flex: 1;
}

.option-title {
  font-size: 16px;
  font-weight: 600;
  color: #303133;
  margin-bottom: 4px;
  transition: color 0.3s ease;
}

.option-description {
  font-size: 14px;
  color: #909399;
  line-height: 1.4;
  transition: color 0.3s ease;
}

.option-badge {
  margin-left: 12px;
}

.connection-status {
  display: flex;
  align-items: center;
  gap: 12px;
  min-height: 42px;
  padding: 0 12px;
  border: 1px solid rgba(148, 163, 184, 0.22);
  border-radius: 8px;
  background: rgba(10, 15, 24, 0.86);
  color: #334155;
}

.status-chip {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 6px 10px;
  border-radius: 999px;
  background: #ecfdf3;
  color: #166534;
  font-size: 13px;
  line-height: 1;
  flex-shrink: 0;
}

.status-chip__dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #22c55e;
}

.status-meta {
  font-size: 13px;
  color: #64748b;
}

.disconnect-btn {
  margin-left: auto;
}

.terminal-empty-state {
  flex: 1;
  min-height: 0;
  width: 100%;
  height: 100%;
  border-radius: 8px;
  border: 1px dashed #cbd5e1;
  background: rgba(255, 255, 255, 0.82);
  display: flex;
  align-items: center;
  justify-content: center;

  :deep(.el-empty) {
    width: 100%;
    height: 100%;
    margin: 0;
    padding: 0;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  :deep(.el-empty__description) {
    margin-top: 12px;
  }
}

.terminal-panel {
  flex: 1;
  min-width: 0;
  min-height: 0;
  display: flex;
  width: 100%;
  overflow: hidden;
}

.terminal-wrapper {
  flex: 1;
  min-width: 0;
  min-height: 0;
  width: 100%;
  border-radius: 8px;
  overflow: hidden;
  border: 1px solid #dbe3ee;
  box-shadow: 0 14px 32px rgba(15, 23, 42, 0.08);
  background: #000;
  display: flex;
}

.terminal-wrapper > * {
  flex: 1;
  min-width: 0;
  min-height: 0;
}

.vnc-placeholder {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
}

@keyframes slideDown {
  from {
    opacity: 0;
    transform: translateY(-20px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@media (max-width: 768px) {
  .connection-options {
    grid-template-columns: 1fr;
    gap: 16px;
  }

  .connection-option {
    padding: 16px;
  }

  .dialog-footer {
    flex-direction: column;
    align-items: stretch;
  }

  .connect-button {
    width: 100%;
  }
}

@media (prefers-color-scheme: dark) {
  .connection-option {
    background: #2c3e50;
    border-color: #34495e;
    color: #ecf0f1;

    &:hover {
      border-color: #667eea;
    }

    &.disabled {
      background: #34495e;
    }
  }

  .option-title {
    color: #ecf0f1;
  }

  .option-description {
    color: #bdc3c7;
  }

  .status-meta {
    color: #cbd5e1;
  }
}
</style>
