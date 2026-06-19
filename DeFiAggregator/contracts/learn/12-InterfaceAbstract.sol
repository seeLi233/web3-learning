// SPDX-License-Identifier: SEE LICENSE IN LICENSE
pragma solidity ^0.8.0;

// =============================================
// 12-InterfaceAbstract.sol — interface 和 abstract 对比
// =============================================

// ----- 1. 接口定义 -----
interface IVehicle {
    // 所有函数必须是 external，不能有实现
    function start() external;
    function stop() external;
    function getSpeed() external view returns (uint256);

    // 可以有事件
    event EngineStarted();
    event EngineStopped();
}

// ----- 2. 抽象合约 — 提供部分实现 -----
abstract contract VehicleBase is IVehicle {
    uint256 public speed;
    bool public isRunning;

    // ✅ 实现接口的部分函数
    function getSpeed() external view override returns (uint256) {
        return speed;
    }

    function start() external override {
        isRunning = true;
        emit EngineStarted();
    }

    // ✅ 添加公共 modifier 供子合约使用
    modifier onlyWhenRunning() {
        require(isRunning, "Engine not running");
        _;
    }

    // ❌ stop() 没有实现 → 留给子合约
    function stop() external virtual override;
}

// ----- 3. 具体实现合约 -----
contract Car is VehicleBase {
    function stop() external override {
        require(isRunning, "Already stopped");
        isRunning = false;
        speed = 0;
        emit EngineStopped();
    }

    function accelerate(uint256 _speed) external onlyWhenRunning {
        speed = _speed;
    }
}

// ----- 4. 接口可以多重继承接口 -----
interface IERC20 {
    function totalSupply() external view returns (uint256);
    function transfer(address to, uint256 amount) external returns (bool);
    event Transfer(address indexed from, address indexed to, uint256 value);
}

interface IERC20Metadata is IERC20 {
    function name() external view returns (string memory);
    function symbol() external view returns (string memory);
    function decimals() external view returns (uint8);
}

interface IERC20Permit is IERC20 {
    function permit(
        address owner,
        address spender,
        uint256 value,
        uint256 deadline,
        uint8 v,
        bytes32 r,
        bytes32 s
    ) external;
}

// 一个合约可以实现多个接口
abstract contract MyAdvancedToken is IERC20, IERC20Metadata, IERC20Permit {
    // ... 实现所有接口的函数
    // （这里只做演示，具体实现在 DeFiToken 中）
}