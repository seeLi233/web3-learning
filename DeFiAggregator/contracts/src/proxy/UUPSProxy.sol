// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "@openzeppelin/contracts/proxy/ERC1967/ERC1967Proxy.sol";

/// @notice 简单的 ERC1967Proxy 包装合约
/// @dev 让 Hardhat 编译并生成 ERC1967Proxy 的 artifact
contract UUPSProxy is ERC1967Proxy {
    constructor(address implementation, bytes memory data)
        ERC1967Proxy(implementation, data)
    {}
}
