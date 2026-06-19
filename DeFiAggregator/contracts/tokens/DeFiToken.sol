// SPDX-License-Identifier: SEE LICENSE IN LICENSE
pragma solidity ^0.8.0;

// =============================================
// DeFiToken.sol — 继承 ERC20 + 铸造 + 燃烧
// 基于 OpenZeppelin 实现
// =============================================

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import "@openzeppelin/contracts/token/ERC20/extensions/ERC20Burnable.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

/// @title DeFi Aggregator Governance Token
/// @notice 可铸造、可燃烧的治理代币
/// @dev 继承链: DeFiToken → ERC20Burnable → ERC20 → IERC20 → IERC20Metadata
contract DeFiToken is ERC20, ERC20Burnable, Ownable {
    // ===== 错误定义 =====
    error DeFiToken__MintToZeroAddress();
    error DeFiToken__MintZeroAmount();

    // ===== 事件 =====
    event TokensMinted(address indexed to, uint256 amount);
    event TokensBurned(address indexed from, uint256 amount);

    // ===== 构造函数 =====
    /// @param name_ 代币全名，如 "DeFi Aggregator Token"
    /// @param symbol_ 代币简称，如 "DEFI"
    /// @param initialSupply_ 初始供应量（已包含 decimals）
    constructor(string memory name_, string memory symbol_, uint256 initialSupply_) ERC20(name_, symbol_) Ownable(msg.sender) {
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