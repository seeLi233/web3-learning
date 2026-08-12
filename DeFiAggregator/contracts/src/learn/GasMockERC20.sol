// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title MockERC20
 * @notice 最小化 ERC20 实现——专为 GasOptimization 测试使用
 * @dev 为什么不用 OpenZeppelin 的完整 ERC20？
 *      → 这个测试只需要 mint + transfer + transferFrom + approve + balanceOf
 *      → 最小实现 = 更快编译 + 更小字节码 + 更易读
 *
 * 为什么 _mint 做成 external？
 * → 测试中需要给不同用户发代币模拟真实场景
 * → 正式合约不会有这种函数——这是测试专用
 */
contract GasMockERC20 {
    // 为什么 mapping 的 key 和 value 都是 address→uint256？
    // → ERC20 标准：balanceOf 查余额，allowance 查授权额度
    mapping(address=>uint256) public balanceOf;
    mapping(address=>mapping(address => uint256)) public allowance;

    uint256 public totalSupply;
    string public name;
    string public symbol;
    uint8 public decimals;
    
    // 构造函数：设置代币基本信息
    // 为什么 name/symbol 用 memory？
    // → 构造参数只需要在部署时用一次，memory 就够了
    constructor(string memory _name, string memory _symbol, uint8 _decimals) {
        name = _name;
        symbol = _symbol;
        decimals = _decimals;
    }

    // 铸造新代币——测试专用
    function mint(address to, uint256 amount) external {
        // 为什么用 unchecked？总供应量在测试中不会超过 uint256 上限
        unchecked {
            totalSupply += amount;
            balanceOf[to] += amount;
        }
    }

    function transfer(address to, uint256 amount) external returns (bool) {
        // 为什么先检查余额？
        // → CEI 模式——实际生产代码应该用 if+revert，这里为简洁用 require
        require(balanceOf[msg.sender] >= amount, "ERC20: insufficient balance");
        unchecked {
            balanceOf[msg.sender] -= amount;
            balanceOf[to] += amount;
        }
        return true;
    }

    function approve(address spender, uint256 amount) external returns (bool) {
        allowance[msg.sender][spender] = amount;
        return true;
    }

    function transferFrom(address from, address to, uint256 amount) external returns (bool) {
        require(balanceOf[from] >= amount, "ERC20: insufficient balance");
        require(allowance[from][msg.sender] >= amount, "ERC20: insufficient allowance");
        unchecked {
            balanceOf[from] -= amount;
            balanceOf[to] += amount;
        }
        return true;
    }
}