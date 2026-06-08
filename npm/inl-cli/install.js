#!/usr/bin/env node
// inl-cli postinstall: 装完子包后, 复制对应平台的二进制到 ./bin/inl
//
// 用户运行 `npm install -g inl-cli` 时:
//   1. npm 装 inl-cli 本身 + optionalDependencies 中的 1 个平台子包
//   2. npm 触发本 postinstall
//   3. 本脚本根据 process.platform/arch 找到对应子包的二进制, 复制到 ./bin/inl
//   4. 后续 `inl` 命令由 bin/inl.js wrapper 调用本地的 ./bin/inl

const fs = require('fs');
const path = require('path');
const { platform, arch } = process;

const platformMap = {
  'linux-x64':    '@inl/cli-linux-x64',
  'linux-arm64':  '@inl/cli-linux-arm64',
  'darwin-x64':   '@inl/cli-darwin-x64',
  'darwin-arm64': '@inl/cli-darwin-arm64',
  'win32-x64':    '@inl/cli-win32-x64',
  'win32-arm64':  '@inl/cli-win32-arm64',
};

const key = `${platform}-${arch}`;
const pkg = platformMap[key];
if (!pkg) {
  console.error(`❌ inl-cli: 不支持的平台 ${key}`);
  console.error(`   支持: ${Object.keys(platformMap).join(', ')}`);
  process.exit(1);
}

try {
  // require.resolve 找到子包入口, 然后定位到其 bin/inl
  const pkgEntry = require.resolve(`${pkg}/package.json`);
  const pkgDir = path.dirname(pkgEntry);
  const src = path.join(pkgDir, 'bin', 'inl');
  const dst = path.join(__dirname, 'bin', 'inl');
  fs.mkdirSync(path.dirname(dst), { recursive: true });
  fs.copyFileSync(src, dst);
  fs.chmodSync(dst, 0o755);
  console.log(`✅ inl-cli: 已安装 (${key})`);
} catch (e) {
  console.error(`❌ inl-cli: 找不到 ${pkg} (npm install 是否成功?)`);
  console.error(`   原始错误: ${e.message}`);
  process.exit(1);
}
