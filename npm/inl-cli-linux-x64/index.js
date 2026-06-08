// @inl/cli-linux-x64 入口
//
// re-export: 子包使用者 (主要是 inl-cli 的 install.js) 可通过 require.resolve
// 找到本入口, 然后定位到 ./bin/inl 真实二进制。

module.exports = {
  name: '@inl/cli-linux-x64',
  binary: require('path').join(__dirname, 'bin', 'inl'),
};
