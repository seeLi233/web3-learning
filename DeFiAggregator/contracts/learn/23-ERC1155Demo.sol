// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/// @title 手写最简 ERC1155 — 理解底层原理
/// @dev 这是一个教学用合约，只实现核心逻辑，不遵循完整标准
contract MinimalERC1155 {

    // ========== 核心存储 ==========

    // ⭐ 核心数据结构：双层嵌套 mapping
    // _balances[id][owner] = amount
    // 问题："owner 拥有多少 id 的代币？"
    mapping (uint256 => mapping (address => uint256)) private _balances;

    // 全量授权：operator 可以操作 owner 的所有代币
    // _operatorApprovals[owner][operator] = true/false
    mapping (address => mapping (address => bool)) private _operatorApprovals;

    // 每个 id 的 URI（元数据链接）
    mapping (uint256 => string) private _tokenURIs;

    // ========== 事件 ==========

    event TransferSingle(address indexed operator, address indexed from, address indexed to, uint256 id, uint value);

    event TransferBatch(address indexed operator, address indexed from, address indexed to, uint256[] ids, uint256[] values);

    event ApprovalForAll(address indexed owner, address indexed operator, bool approved);

    // ========== 自定义错误 ==========

    error ERC1155__ZeroAddress();
    error ERC1155__InsufficientBalance(address sender, uint256 balance, uint256 needed, uint256 id);
    error ERC1155__NotAuthorized(address caller, address owner);
    error ERC1155__InvalidArrayLength();
    error ERC1155__TransferToZeroAddress();

    // ========== 查询函数 ==========

    /// @notice 查询某人拥有某个 id 的余额
    function balanceOf(address account, uint256 id) public view returns (uint256) {
        return _balances[id][account];
    }

    /// @notice 批量查询余额 — ERC1155 的核心特色功能
    /// @dev accounts 和 ids 长度必须一致
    function balanceOfBatch(address[] calldata accounts, uint256[] calldata ids) public view returns (uint256[] memory) {
        if (accounts.length != ids.length) {
            revert ERC1155__InvalidArrayLength();
        }

        uint256[] memory batchBalances = new uint256[](accounts.length);
        for (uint256 i = 0; i < accounts.length; i++) {
            batchBalances[i] = _balances[ids[i]][accounts[i]];
        }
        return batchBalances;
    }

    /// @notice 查询是否全量授权
    function isApprovedForAll(address account, address operator) public view returns (bool) {
        return _operatorApprovals[account][operator];
    }

    // ========== 授权函数 ==========

    /// @notice ERC1155 只有全量授权，没有单 token 授权
    function setApprovalForAll(address operator, bool approved) public {
        _operatorApprovals[msg.sender][operator] = approved;
        emit ApprovalForAll(msg.sender, operator, approved);
    }

    // ========== 转账函数 ==========

    /// @notice 安全转账单个 id 的代币
    function safeTransferFrom(address from, address to, uint256 id, uint256 amount, bytes calldata data) public {
        // Step 1: 权限检查
        if (from != msg.sender && !isApprovedForAll(from, msg.sender)) {
            revert ERC1155__NotAuthorized(msg.sender, from);
        }

        if (to == address(0)) {
            revert ERC1155__ZeroAddress();
        }

        // Step 2: 执行转账
        _transferSingle(from, to, id, amount);

        // Step 3: 安全检查（合约接收方必须实现 IERC1155Receiver）
        _doSafeTransferAcceptanceCheck(msg.sender, from, to, id, amount, data);
    }

    /// @notice 安全批量转账 — ERC1155 的核心优势
    function safeBatchTransferFrom(address from, address to, uint256[] calldata ids, uint256[] calldata amounts, bytes calldata data) public {
        // Step 1: 权限检查
        if (msg.sender != from && !isApprovedForAll(from, msg.sender)) {
            revert ERC1155__NotAuthorized(msg.sender, from);
        }

        if (to == address(0)) {
            revert ERC1155__ZeroAddress();
        }

        if (ids.length != amounts.length) {
            revert ERC1155__InvalidArrayLength();
        }

        // Step 2: 执行批量转账
        _transferBatch(from , to, ids, amounts);

        // Step 3: 批量安全检查
        _doSafeBatchTransferAcceptanceCheck(msg.sender, from, to, ids, amounts, data);
    }

    // ========== 内部铸造函数 ==========

    /// @notice 铸造新代币（单 id）
    function _mint(address to, uint256 id, uint256 amount, bytes memory data) internal {
        if (to == address(0)) {
            revert ERC1155__ZeroAddress();
        }

        _balances[id][to] += amount;
        emit TransferSingle(msg.sender, address(0), to, id, amount);

        _doSafeTransferAcceptanceCheck(msg.sender, address(0), to, id, amount, data);
    }

    /// @notice 批量铸造新代币
    function _mintBatch(address to, uint256[] memory ids, uint256[] memory amounts, bytes calldata data) internal {
        if (to == address(0)) {
            revert ERC1155__ZeroAddress();
        }
        if (ids.length != amounts.length) {
            revert ERC1155__InvalidArrayLength();
        }

        for (uint256 i = 0; i < ids.length; i++) {
            _balances[ids[i]][to] += amounts[i];
        }
        emit TransferBatch(msg.sender, address(0), to, ids, amounts);

        _doSafeBatchTransferAcceptanceCheck(msg.sender, address(0), to, ids, amounts, data);
    }

    /// @notice 燃烧代币
    function _burn(address from, uint256 id, uint256 amount) internal {
        uint256 fromBalance = _balances[id][from];
        if (fromBalance < amount) {
            revert ERC1155__InsufficientBalance(from, fromBalance, id, amount);
        }

        _balances[id][from] -= amount;
        emit TransferSingle(msg.sender, from, address(0), amount, id);
    }

    // ========== 内部转账核心 ==========
    function _transferSingle(address from, address to, uint256 id, uint256 amount) private {
        uint256 fromBalance = _balances[id][from];
        if (fromBalance < amount) {
            revert ERC1155__InsufficientBalance(from, fromBalance, amount, id);
        }

        // CEI 模式：先检查，再修改状态
        _balances[id][from] -= amount;
        _balances[id][to] += amount;

        emit TransferSingle(msg.sender, from, to, id, amount);
    }

    function _transferBatch(address from, address to, uint256[] memory ids, uint256[] memory amounts) private {
        // 循环执行所有转账
        for(uint256 i = 0; i < ids.length; i++) {
            uint256 id = ids[i];
            uint256 amount = amounts[i];

            uint256 fromBalance = _balances[id][from];
            if(fromBalance < amount) {
                revert ERC1155__InsufficientBalance(from, fromBalance, amount, id);
            }

            _balances[id][from] -= amount;
            _balances[id][to] += amount;
        }

        emit TransferBatch(msg.sender, from, to, ids, amounts);
    }

    // ========== 安全检查 ==========

    /// @notice 检查接收方是否能处理 ERC1155 代币
    function _doSafeTransferAcceptanceCheck(address operator, address from, address to, uint256 id, uint256 amount, bytes memory data) private {
        // 如果接收方是合约，必须实现 IERC1155Receiver 接口
        if (to.code.length > 0) {
            try IERC1155Receiver(to).onERC1155Received(operator, from, id, amount, data) returns (bytes4 response) {
                // 返回值必须是 0xf23a6e61
                if (response != IERC1155Receiver.onERC1155Received.selector) {
                    revert("ERC1155: ERC1155Receiver rejected tokens");
                }
            } catch Error(string memory reason) {
                revert(reason);
            } catch {
                revert("ERC1155: transfer to non-ERC1155Receiver implementer");
            }
        }
    }

    function _doSafeBatchTransferAcceptanceCheck(address operator, address from, address to, uint256[] memory ids, uint256[] memory amounts, bytes calldata data) private {
        if (to.code.length > 0) {
            try IERC1155Receiver(to).onERC1155BatchReceived(
                operator, from, ids, amounts, data
            ) returns (bytes4 response) {
                // 返回值必须是 0xbc197c81
                if (response != IERC1155Receiver.onERC1155BatchReceived.selector) {
                    revert("ERC1155: ERC1155Receiver rejected tokens");
                }
            } catch Error(string memory reason) {
                revert(reason);
            } catch {
                revert("ERC1155: transfer to non-ERC1155Receiver implementer");
            }
        }
    }
}

// ========== IERC1155Receiver 接口定义 ==========
// （正常应该 import，这里为演示手写）

interface IERC1155Receiver {
    function onERC1155Received(address operator, address from, uint256 id, uint256 value, bytes calldata data) external returns (bytes4);

    function onERC1155BatchReceived(address operator, address from, uint256[] calldata ids, uint256[] calldata values, bytes calldata data) external returns (bytes4);
}