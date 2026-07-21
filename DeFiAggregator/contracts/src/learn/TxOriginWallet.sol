// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title TxOriginWallet
 * @notice 🔴 漏洞合约：用 tx.origin 做身份认证
 * @dev 任何人都能通过钓鱼合约偷走 owner 的钱
 */
contract TxOriginWallet {
    address public owner;

    constructor() payable {
        owner = msg.sender;
    }

    // 🔴 漏洞点：用 tx.origin 而不是 msg.sender
    function withdraw() external {
        require(tx.origin == owner, "Not owner"); // ← 致命漏洞！
        (bool ok, ) = msg.sender.call{value:address(this).balance}("");
        require(ok, "Transfer failed");
    }

    function getBalance() external view returns (uint256) {
        return address(this).balance;
    }

    receive() external payable {}
}