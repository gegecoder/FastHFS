import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";

// 对外访问站点：构建产物直接嵌入 Go 二进制（backend/fileserver/www/dist）
export default defineConfig({
  // 使用相对路径，配合服务端注入的 <base href> 支持反向代理子路径部署
  base: "./",
  plugins: [vue()],
  server: {
    host: "127.0.0.1",
    port: 9246,
    strictPort: true,
  },
  build: {
    outDir: "../backend/fileserver/www/dist",
    emptyOutDir: true,
    chunkSizeWarningLimit: 2000,
  },
});
