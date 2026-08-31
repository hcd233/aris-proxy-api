import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "export",
  basePath: "/web",
  trailingSlash: true,
  images: {
    unoptimized: true,
  },
  turbopack: {
    // 仓库根目录存在一个空的游离 package-lock.json，Turbopack 的多 lockfile
    // 推断会把整个 Go 仓库当成项目根并纳入监听。前端是自包含 npm 工程，
    // 显式锚定到 web/ 以消除该警告并缩小监听范围。
    root: __dirname,
  },
};

export default nextConfig;
