<template>
  <div ref="xtermRef" class="xterm-host">
    <div v-show="scrollbar.visible" class="xterm-scrollbar" @pointerdown.stop="handleTrackPointerDown">
      <div
        class="xterm-scrollbar__thumb"
        :class="{ 'is-dragging': scrollbarDragging }"
        :style="{
          height: `${scrollbar.thumbHeight}px`,
          transform: `translateY(${scrollbar.thumbTop}px)`
        }"
        @pointerdown.stop="handleThumbPointerDown"
      />
    </div>
  </div>
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
    prefix: PrefixConfig | undefined
    apiBase: string
  }>(),
  {
    resource_id: "1",
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
const scrollbar = ref({
  visible: false,
  thumbHeight: 0,
  thumbTop: 0
})
const scrollbarDragging = ref(false)
let resizeObserver: ResizeObserver | undefined
let viewportResizeObserver: ResizeObserver | undefined
let resizeHandler: ReturnType<typeof _.debounce> | undefined
let dataDisposer: { dispose: () => void } | undefined
let renderDisposer: { dispose: () => void } | undefined
let scrollDisposer: { dispose: () => void } | undefined
let viewportEl: HTMLElement | undefined
let scrollbarFrame = 0
let dragStartY = 0
let dragStartScrollTop = 0

const scheduleInitialFit = () => {
  nextTick(() => {
    resizeTerminal()
    bindScrollbarEvents()
    syncScrollbar()
    window.requestAnimationFrame(() => {
      resizeTerminal()
      syncScrollbar()
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
  renderDisposer = xterm.value.onRender(() => {
    syncScrollbar()
  })
  scrollDisposer = xterm.value.onScroll(() => {
    syncScrollbar()
  })

  socket.value = new WebSocket(
    `${props.prefix.wsServer}${props.apiBase}/terminal/ws?resource_id=${props.resource_id}&cols=${xterm.value.cols}&rows=${xterm.value.rows}`
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

      socket.value!.send(JSON.stringify(message))
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
    const content = JSON.parse(msg.data as string) as { data: string }
    xterm.value?.write(content.data)
  }
}

const resizeTerminal = () => {
  if (xterm.value && fitAddon.value) {
    fitAddon.value.fit()
    syncScrollbar()

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

const getViewportEl = () => {
  if (!viewportEl) {
    viewportEl = xtermRef.value?.querySelector(".xterm-viewport") as HTMLElement | undefined
  }
  return viewportEl
}

const syncScrollbar = () => {
  if (scrollbarFrame) {
    window.cancelAnimationFrame(scrollbarFrame)
  }

  scrollbarFrame = window.requestAnimationFrame(() => {
    scrollbarFrame = 0
    const host = xtermRef.value
    const viewport = getViewportEl()
    if (!host || !viewport) {
      scrollbar.value = {
        visible: false,
        thumbHeight: 0,
        thumbTop: 0
      }
      return
    }

    const scrollHeight = viewport.scrollHeight
    const clientHeight = viewport.clientHeight
    const trackHeight = Math.max(host.clientHeight - 8, 0)
    const canScroll = scrollHeight > clientHeight + 1 && trackHeight > 0
    if (!canScroll) {
      scrollbar.value = {
        visible: false,
        thumbHeight: 0,
        thumbTop: 0
      }
      return
    }

    const thumbHeight = Math.max(24, Math.round((clientHeight / scrollHeight) * trackHeight))
    const maxThumbTop = Math.max(trackHeight - thumbHeight, 0)
    const maxScrollTop = Math.max(scrollHeight - clientHeight, 1)
    const thumbTop = Math.round((viewport.scrollTop / maxScrollTop) * maxThumbTop)

    scrollbar.value = {
      visible: true,
      thumbHeight,
      thumbTop
    }
  })
}

const bindScrollbarEvents = () => {
  const viewport = getViewportEl()
  if (!viewport) {
    return
  }

  viewport.addEventListener("scroll", syncScrollbar)

  if (window.ResizeObserver) {
    viewportResizeObserver?.disconnect()
    viewportResizeObserver = new ResizeObserver(() => {
      syncScrollbar()
    })
    viewportResizeObserver.observe(viewport)
  }
}

const scrollViewportFromTrack = (clientY: number) => {
  const host = xtermRef.value
  const viewport = getViewportEl()
  if (!host || !viewport || !scrollbar.value.visible) {
    return
  }

  const rect = host.getBoundingClientRect()
  const trackTop = rect.top + 4
  const trackHeight = Math.max(host.clientHeight - 8, 0)
  const nextThumbTop = Math.min(
    Math.max(clientY - trackTop - scrollbar.value.thumbHeight / 2, 0),
    Math.max(trackHeight - scrollbar.value.thumbHeight, 0)
  )
  const maxScrollTop = Math.max(viewport.scrollHeight - viewport.clientHeight, 0)
  const maxThumbTop = Math.max(trackHeight - scrollbar.value.thumbHeight, 1)
  viewport.scrollTop = Math.round((nextThumbTop / maxThumbTop) * maxScrollTop)
  syncScrollbar()
}

const handleTrackPointerDown = (event: PointerEvent) => {
  scrollViewportFromTrack(event.clientY)
}

const handleThumbPointerDown = (event: PointerEvent) => {
  const viewport = getViewportEl()
  if (!viewport) {
    return
  }

  scrollbarDragging.value = true
  dragStartY = event.clientY
  dragStartScrollTop = viewport.scrollTop
  window.addEventListener("pointermove", handleThumbPointerMove)
  window.addEventListener("pointerup", handleThumbPointerUp, { once: true })
}

const handleThumbPointerMove = (event: PointerEvent) => {
  const host = xtermRef.value
  const viewport = getViewportEl()
  if (!host || !viewport || !scrollbar.value.visible) {
    return
  }

  const trackHeight = Math.max(host.clientHeight - 8, 0)
  const maxThumbTop = Math.max(trackHeight - scrollbar.value.thumbHeight, 1)
  const maxScrollTop = Math.max(viewport.scrollHeight - viewport.clientHeight, 0)
  const scrollDelta = ((event.clientY - dragStartY) / maxThumbTop) * maxScrollTop
  viewport.scrollTop = Math.round(dragStartScrollTop + scrollDelta)
  syncScrollbar()
}

const handleThumbPointerUp = () => {
  scrollbarDragging.value = false
  window.removeEventListener("pointermove", handleThumbPointerMove)
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
  viewportResizeObserver?.disconnect()
  viewportResizeObserver = undefined

  if (viewportEl) {
    viewportEl.removeEventListener("scroll", syncScrollbar)
  }
  viewportEl = undefined

  if (scrollbarFrame) {
    window.cancelAnimationFrame(scrollbarFrame)
    scrollbarFrame = 0
  }
  window.removeEventListener("pointermove", handleThumbPointerMove)
  window.removeEventListener("pointerup", handleThumbPointerUp)
  scrollbarDragging.value = false

  if (resizeHandler) {
    window.removeEventListener("resize", resizeHandler)
    document.removeEventListener("fullscreenchange", resizeHandler)
    resizeHandler.cancel?.()
    resizeHandler = undefined
  }

  dataDisposer?.dispose()
  dataDisposer = undefined
  renderDisposer?.dispose()
  renderDisposer = undefined
  scrollDisposer?.dispose()
  scrollDisposer = undefined

  if (pingInterval.value) {
    clearInterval(pingInterval.value)
    pingInterval.value = undefined
  }

  socket.value?.close()
  socket.value = undefined
  scrollbar.value = {
    visible: false,
    thumbHeight: 0,
    thumbTop: 0
  }

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
  position: relative;
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
  position: absolute !important;
  inset: 0 !important;
  width: 100% !important;
  height: 100% !important;
  min-width: 0;
  min-height: 0;
  box-sizing: border-box;
  padding: 6px 10px 6px 8px;
}

// NOTE: xterm.js 通过 JS 内联样式强制设置 overflow-y: scroll，CSS 无法覆盖，
// 通过 right: -20px 将原生滚动条推出 overflow:hidden 的裁剪区域来隐藏
.xterm-host :deep(.xterm-viewport) {
  position: absolute !important;
  top: 0 !important;
  left: 0 !important;
  right: -20px !important;
  bottom: 0 !important;
  width: auto !important;
}

.xterm-scrollbar {
  position: absolute;
  top: 4px;
  right: 3px;
  bottom: 4px;
  z-index: 5;
  width: 6px;
  border-radius: 999px;
  background: transparent;
  cursor: default;
}

.xterm-scrollbar__thumb {
  width: 100%;
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.46);
  cursor: grab;
  transition: background-color 0.12s ease;
}

.xterm-scrollbar__thumb:hover,
.xterm-scrollbar__thumb.is-dragging {
  background: rgba(203, 213, 225, 0.72);
}

.xterm-scrollbar__thumb.is-dragging {
  cursor: grabbing;
}
</style>
