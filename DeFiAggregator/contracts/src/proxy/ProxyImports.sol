// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

// 导入 OZ 代理合约，确保 Hardhat 编译其 artifact
// 测试中需要使用 ethers.deployContract("ERC1967Proxy", ...) 部署代理
import "@openzeppelin/contracts/proxy/ERC1967/ERC1967Proxy.sol";
