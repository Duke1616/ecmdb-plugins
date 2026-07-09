<template>
  <div ref="xtermRef" class="xterm-host" />
</template>

<script lang="ts" setup>
import { nextTick, onBeforeUnmount, onDeactivated, onMounted, ref } from "vue"

import "@xterm/xterm/css/xterm.css"
import { ITerminalInitOnlyOptions, ITerminalOptions, Terminal } from "@xterm/xterm"
import { FitAddon } from "@xterm/addon-fit"
import _ from "lodash"
import { ElMessage } from "element-plus"
import type { PrefixConfig } from "./utils/prefix-config"

const props = withDefaults(
  defineProps<{
    resource_id: string
    sessionId?: string
    prefix: PrefixConfig | undefined
    apiBase: string
  }>(),
  {
    resource_id: "1",
    sessionId: "",
    apiBase: ""
  }
)

defineOptions({
  name: "IXterm"
})

const xtermRef = ref<HTMLElement>()
const fitAddon = ref<FitAddon>()
const xterm = ref<Terminal | null>(null)
const socket = ref<WebSocket>()
let resizeObserver: ResizeObserver | undefined
let resizeHandler: ReturnType<typeof _.debounce> | undefined
let dataDisposer: { dispose: () => void } | undefined

const scheduleInitialFit = () => {
  nextTick(() => {
    resizeTerminal()
    window.requestAnimationFrame(() => {
      resizeTerminal()
    })
  })
}

const initXterm = () => {
  if (!xtermRef.value || !props.prefix?.wsServer) {
    return
  }

  const options = ref<ITerminalOptions & ITerminalInitOnlyOptions>({
    fontSize: 14,
    fontFamily: 'monaco, Consolas, "Lucida Console", monospace',
    convertEol: false, //启用时，光标将设置为下一行的开头
    scrollback: 2000, //终端中的回滚量
    disableStdin: false, //是否应禁用输入
    cursorStyle: "underline", //光标样式
    cursorBlink: true, //光标闪烁
    theme: {
      foreground: "#ECECEC", //字体
      background: "#000000", //背景色
      cursor: "help" //设置光标
    }
  })

  xterm.value = new Terminal(options.value)
  xterm.value.open(xtermRef.value as HTMLElement)
  fitAddon.value = new FitAddon()
  xterm.value.loadAddon(fitAddon.value)
  fitAddon.value.fit()
  xterm.value.focus()

  socket.value = new WebSocket(
    `${props.prefix.wsServer}${props.apiBase}/terminal/ws?session_id=${props.sessionId || props.resource_id}&cols=${xterm.value.cols}&rows=${xterm.value.rows}`
  )

  socketOnClose()
  socketOnOpen()
  socketOnMessage()
  socketOnError()

  // 发送数据
  dataDisposer = xterm.value?.onData(function (data: string) {
    const message = {
      operation: "stdin",
      data: data
    }

    if (socket.value?.readyState === WebSocket.OPEN) {
      socket.value.send(JSON.stringify(message))
    }
  })

  bindResizeEvents()
  scheduleInitialFit()
}

const pingInterval = ref()
const socketOnOpen = () => {
  socket.value!.onopen = () => {
    pingInterval.value = setInterval(() => {
      const message = {
        operation: "ping",
        data: ""
      }

      if (socket.value?.readyState === WebSocket.OPEN) {
        socket.value.send(JSON.stringify(message))
      }
    }, 10000)
  }
}

const socketOnClose = () => {
  socket.value!.onclose = () => {
    xterm.value?.writeln("connection is closed.")
    if (pingInterval.value) {
      clearInterval(pingInterval.value)
    }
  }
}

const socketOnError = () => {
  socket.value!.onerror = () => {
    const errorMsg = "WebSocket 连接错误"
    xterm.value?.writeln(`websocket error: \x1B[1;3;31m${errorMsg}\x1B[0m `)
    ElMessage.error("错误：连接失败")
  }
}

const socketOnMessage = () => {
  socket.value!.onmessage = (msg: MessageEvent) => {
    const content = JSON.parse(msg.data as string) as { operation?: string; data: string }
    if (content.operation === "pong") {
      return
    }
    xterm.value?.write(content.data)
  }
}

const resizeTerminal = () => {
  if (xterm.value && fitAddon.value) {
    fitAddon.value.fit()

    const cols = xterm.value.cols
    const rows = xterm.value.rows
    const terminalSize = {
      operation: "resize",
      cols,
      rows
    }
    if (socket.value?.readyState === WebSocket.OPEN) {
      socket.value.send(JSON.stringify(terminalSize))
    }
  }
}

const bindResizeEvents = () => {
  resizeHandler = _.debounce(() => {
    resizeTerminal()
  }, 80)

  window.addEventListener("resize", resizeHandler)
  document.addEventListener("fullscreenchange", resizeHandler)

  if (window.ResizeObserver && xtermRef.value) {
    resizeObserver = new ResizeObserver(() => {
      resizeHandler?.()
    })
    resizeObserver.observe(xtermRef.value)
    if (xtermRef.value.parentElement) {
      resizeObserver.observe(xtermRef.value.parentElement)
    }
  }
}

const cleanup = () => {
  resizeObserver?.disconnect()
  resizeObserver = undefined

  if (resizeHandler) {
    window.removeEventListener("resize", resizeHandler)
    document.removeEventListener("fullscreenchange", resizeHandler)
    resizeHandler.cancel?.()
    resizeHandler = undefined
  }

  dataDisposer?.dispose()
  dataDisposer = undefined

  if (pingInterval.value) {
    clearInterval(pingInterval.value)
    pingInterval.value = undefined
  }

  socket.value?.close()
  socket.value = undefined

  // NOTE: FitAddon 在终端未完全初始化就销毁时可能抛出 "addon has not been loaded" 错误，
  // 这是 xterm.js 的内部校验，防御性捕获即可，不影响实际清理逻辑
  try {
    xterm.value?.dispose()
  } catch {
    // 忽略 addon 未加载时的销毁异常
  }
  xterm.value = null
  fitAddon.value = undefined
}

onMounted(() => {
  initXterm()
})

onBeforeUnmount(() => {
  cleanup()
})

onDeactivated(() => {
  cleanup()
})
</script>

<style lang="scss" scoped>
.xterm-host {
  display: flex;
  flex: 1;
  width: 100%;
  height: 100%;
  min-width: 0;
  min-height: 0;
  background-color: #000;
  box-sizing: border-box;
  overflow: hidden;
}

.xterm-host :deep(.xterm) {
  flex: 1;
  width: 100% !important;
  height: 100% !important;
  min-width: 0;
  min-height: 0;
  box-sizing: border-box;
  padding: 6px 0 6px 8px;
}

.xterm-host :deep(.xterm-viewport) {
  position: absolute !important;
  top: 0 !important;
  left: 0 !important;
  right: 0 !important;
  bottom: 0 !important;
  width: auto !important;
  scrollbar-width: thin;
  scrollbar-color: rgba(148, 163, 184, 0.46) transparent;
}

.xterm-host :deep(.xterm-viewport::-webkit-scrollbar) {
  width: 8px;
}

.xterm-host :deep(.xterm-viewport::-webkit-scrollbar-track) {
  background: transparent;
}

.xterm-host :deep(.xterm-viewport::-webkit-scrollbar-thumb) {
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.46);
}
</style>
