// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

// =============================================
// DeFiToken.sol — 继承 ERC20 + 铸造 + 燃烧
// 基于 OpenZeppelin 实现
// =============================================

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import "@openzeppelin/contracts/token/ERC20/extensions/ERC20Permit.sol";
import "@openzeppelin/contracts/token/ERC20/extensions/ERC20Votes.sol";
import "@openzeppelin/contracts/token/ERC20/extensions/ERC20Burnable.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

/**
 * @title DeFiToken
 * @notice 功能完备的治理代币
 *
 * 继承链:
 *   ERC20 → ERC20Permit → ERC20Votes → ERC20Burnable → Ownable
 *
 * 功能清单:
 *   ✅ ERC20: transfer / approve / transferFrom
 *   ✅ ERC20Permit: gasless approve，离线签名授权
 *   ✅ ERC20Votes: 治理投票 + 委托 + 历史快照
 *   ✅ ERC20Burnable: 燃烧代币（通缩）
 *   ✅ Ownable: owner 才能铸造
 */
contract DeFiToken is ERC20, ERC20Permit, ERC20Votes, ERC20Burnable, Ownable {
    // ===== 错误定义 =====
    error DeFiToken__MintToZeroAddress();
    error DeFiToken__MintZeroAmount();
    error DeFiToken__BatchLengthMismatch();
    error DeFiToken__EmptyBatch();

    // ===== 事件 =====
    event TokensMinted(address indexed to, uint256 amount);
    event TokensBurned(address indexed from, uint256 amount);

    // ===== 构造函数 =====
    /// @param name_ 代币全名，如 "DeFi Aggregator Token"
    /// @param symbol_ 代币简称，如 "DEFI"
    /// @param initialSupply_ 初始供应量（已包含 decimals）
    constructor(string memory name_, string memory symbol_, uint256 initialSupply_) ERC20(name_, symbol_) ERC20Permit(name_) Ownable(msg.sender) {
        // 检查初始供应商，防止零供应商
        if (initialSupply_ == 0) {
            revert DeFiToken__MintZeroAmount();
        }

        // 铸造初始供应量给合约部署者
        _mint(msg.sender, initialSupply_);
        emit TokensMinted(msg.sender, initialSupply_);
    }

    // ===== 铸造函数 =====
    /// @notice 只有 owner 可以铸造新代币
    /// @param to 接收代币的地址
    /// @param amount 铸造数量
    function mint(address to, uint256 amount) external onlyOwner {
        if (to == address(0)) {
            revert DeFiToken__MintToZeroAddress();
        }
        if (amount == 0) {
            revert DeFiToken__MintZeroAmount();
        }

        _mint(to, amount);
        emit TokensMinted(to, amount);
    }

    // ===== 必须重写的三个函数 =====
    // 因为 ERC20 和 ERC20Votes 都定义了这些函数,
    // Solidity 要求显式重写来解决冲突

    /**
     * @notice 更新函数 — ERC20 和 ERC20Votes 都需要更新余额
     */
    function _update(address from, address to, uint256 value) internal override(ERC20, ERC20Votes) {
        super._update(from, to, value);
    }

    /**
     * @notice nonces — ERC20Permit 和 ERC20Votes 都需要 nonce 管理
     */
    function nonces(address owner) public view override(ERC20Permit, Nonces) returns (uint256) {
        return super.nonces(owner);
    }

    // ===== Gas 优化: 批量操作（面试加分项） =====

    /**
     * @notice 批量转账 — 节省 gas
     * @param recipients 接收者地址数组
     * @param amounts 金额数组（和 recipients 一一对应）
     */
    function batchTransfer(address[] calldata recipients, uint256[] calldata amounts) external returns (bool) {
        if (recipients.length != amounts.length) {
            revert DeFiToken__BatchLengthMismatch();
        }
        if (recipients.length == 0) {
            revert DeFiToken__EmptyBatch();
        }

        for (uint256 i = 0; i < recipients.length; i++) {
            transfer(recipients[i], amounts[i]);
        }
        return true;
    }

    /**
     * @notice 查询链上当前时间（用于投票快照）
     */
    function clock() public view override returns (uint48) {
        return uint48(block.timestamp);
    }

    /**
     * @notice 时钟模式 — 用时间戳而不是区块号
     */
    function CLOCK_MODE() public pure override returns (string memory) {
        return "mode=timestamp";
    }

    // ===== 燃烧函数（重写 ERC20Burnable）=====
    /// @notice 燃烧自己的代币
    /// @param amount 燃烧数量
    function burn(uint256 amount) public override {
        emit TokensBurned(msg.sender, amount);
        super.burn(amount);
    }

    /// @notice 燃烧他人的代币（需要授权）
    /// @param account 被燃烧代币的地址
    /// @param amount 燃烧数量
    function burnFrom(address account, uint256 amount) public override {
        emit TokensBurned(account, amount);
        super.burnFrom(account, amount);
    }
}