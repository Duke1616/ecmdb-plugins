import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [vue()],
  build: {
    lib: {
      entry: resolve(__dirname, 'src/index.ts'),
      name: 'EcmdbPluginBuiltinSsh', // 挂载到 window 上的全局变量名称 (根据命名规范自动推导)
      fileName: (format) => `index.${format}.js`,
      formats: ['umd'] // 打包成 umd 格式即可
    },
    rollupOptions: {
      // 声明外部化依赖，不打包进本 JS，运行时从主前端全局获取
      external: ['vue', 'vue-router', 'pinia', 'element-plus'],
      output: {
        globals: {
          vue: 'Vue',
          'vue-router': 'VueRouter',
          pinia: 'Pinia',
          'element-plus': 'ElementPlus'
        },
        // 确保 CSS 不被分拆，合并输出为 index.css
        assetFileNames: (assetInfo) => {
          if (assetInfo.name === 'style.css') return 'index.css'
          return assetInfo.name
        }
      }
    }
  }
})
