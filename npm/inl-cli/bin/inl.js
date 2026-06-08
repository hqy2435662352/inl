#!/usr/bin/env node
// inl-cli bin 入口: 透传到 install.js 复制的本地二进制
//
// 调用链:
//   $PATH/inl (符号链接) → npm/inl-cli/bin/inl.js (本文件) → npm/inl-cli/bin/inl (实际二进制)
//
// 直接 spawn 真实二进制, 透传所有 argv。

const { spawn } = require('child_process');
const path = require('path');

const realBin = path.join(__dirname, 'inl');
const child = spawn(realBin, process.argv.slice(2), {
  stdio: 'inherit',
});

child.on('exit', (code) => process.exit(code ?? 0));
child.on('error', (e) => {
  console.error(`❌ inl 执行失败: ${e.message}`);
  console.error(`   提示: 请运行 'npm install -g inl-cli' 重新安装`);
  process.exit(1);
});
