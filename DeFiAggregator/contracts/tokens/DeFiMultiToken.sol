// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "@openzeppelin/contracts/token/ERC1155/ERC1155.sol";
import "@openzeppelin/contracts/token/ERC1155/extensions/ERC1155Burnable.sol";
import "@openzeppelin/contracts/token/ERC1155/extensions/ERC1155Supply.sol";
import "@openzeppelin/contracts/access/Ownable.sol";
import "@openzeppelin/contracts/utils/Strings.sol";

/// @title DeFiMultiToken — 基于 ERC1155 的多代币合约
/// @notice 一个合约管理多种代币：FT（同质化）+ NFT（非同质化）+ SFT（半同质化）
contract DeFiMultiToken is ERC1155, ERC1155Burnable, ERC1155Supply, Ownable {

    using Strings for uint256;

    // ========== 状态变量 ==========

    string public name;
    string private _baseURI;

    // 最大供应量（0 表示无限制）
    mapping (uint256 => uint256) public maxSupply;

    // 是否已锁定供应量
    mapping (uint256 => bool) public supplyLocked;

    // 代币 id 的自增计数器
    uint256 private _nextTokenId;

    // id → 是否已创建
    mapping (uint256 => bool) private _tokenExists;

    // ========== 事件 ==========

    event TokenCreated(uint256 indexed id, string name, uint256 maxSupply, address creator);

    event BaseURISet(string oldURI, string newURI);
    event MaxSupplySet(uint256 indexed id, uint256 maxSupply);
    event SupplyLocked(uint256 indexed id, uint256 finalSupply);

    // ========== 自定义错误 ==========

    error DeFiMultiToken__MaxSupplyReached(uint256 id, uint256 maxSupply, uint256 currentSupply, uint256 requested);
    error DeFiMultiToken__TokenDoesNotExist(uint256 id);
    error DeFiMultiToken__TokenAlreadyExists(uint256 id);
    error DeFiMultiToken__SupplyLocked(uint256 id);
    error DeFiMultiToken__InvalidMaxSupply(uint256 id, uint256 currentSupply, uint256 newMaxSupply);
    error DeFiMultiToken__ZeroAddress();
    error DeFiMultiToken__ZeroAmount();
    error DeFiMultiToken__NotTokenOwner(uint256 id, address account, uint256 balance, uint256 required);
    error DeFiMultiToken__ArrayLengthMismatch();

    // ========== 修饰符 ==========

    modifier tokenExists(uint256 id) {
        if (!_tokenExists[id]) revert DeFiMultiToken__TokenDoesNotExist(id);
        _;
    }

    modifier supplyNotLocked(uint256 id) {
        if (supplyLocked[id]) revert DeFiMultiToken__SupplyLocked(id);
        _;
    }

    // ========== 构造函数 ==========

    /// @param _name 代币合集名称
    /// @param baseURI_ 元数据基础 URI（应包含 {id} 占位符）
    constructor(string memory _name, string memory baseURI_) ERC1155(baseURI_) Ownable(msg.sender) {
        name = _name;
        _baseURI = baseURI_;
    }

    // ========== 代币创建 ==========

    /// @notice 创建一种新的代币类型
    /// @param maxSupply_ 最大供应量（0 = 无限制，适合 FT）
    /// @param tokenURI_ 该代币的元数据 URI（可选，传入空字符串则用 baseURI）
    /// @return id 新代币的 id
    function createToken(uint256 maxSupply_, string memory tokenURI_) public onlyOwner returns (uint256) {
        uint256 id = _nextTokenId++;

        _tokenExists[id] = true;
        maxSupply[id] = maxSupply_;

        // 如果指定了独立 URI，触发 URI 事件（客户端可以监听这个事件）
        if (bytes(tokenURI_).length > 0) {
            emit URI(tokenURI_, id);
        }

        emit TokenCreated(id, tokenURI_, maxSupply_, msg.sender);
        return id;
    }

    // ========== 铸造函数 ==========

    /// @notice 铸造同质化代币（一个 id 铸造多个）
    function mint(address to, uint256 id, uint256 amount, bytes memory data) external onlyOwner tokenExists(id) supplyNotLocked(id) {
        if (to == address(0)) revert DeFiMultiToken__ZeroAddress();
        if (amount == 0) revert DeFiMultiToken__ZeroAmount();

        // 检查最大供应量
        if (maxSupply[id] > 0) {
            uint256 currentSupply = totalSupply(id);
            if (currentSupply + amount > maxSupply[id]) {
                revert DeFiMultiToken__MaxSupplyReached(id, maxSupply[id], currentSupply, amount);
            }
        }

        _mint(to, id, amount, data);
    }

    /// @notice 铸造 NFT（一个 id 只铸造 1 个，supply = 1）
    function mintNFT(address to, string memory tokenURI_) external onlyOwner returns (uint256) {
        uint256 id = createToken(1, tokenURI_);
        _mint(to, id, 1, "");
        return id;
    }

    /// @notice 批量铸造（多个 id 一次铸造）
    function mintBatch(address to, uint256[] memory ids, uint256[] memory amounts, bytes memory data) external onlyOwner {
        if (to == address(0)) revert DeFiMultiToken__ZeroAddress();
        if (ids.length != amounts.length) revert DeFiMultiToken__ArrayLengthMismatch();

        for (uint256 i = 0; i < ids.length; i++) {
            uint256 id = ids[i];
            if (!_tokenExists[id]) revert DeFiMultiToken__TokenDoesNotExist(id);
            if (supplyLocked[id]) revert DeFiMultiToken__SupplyLocked(id);

            if (maxSupply[id] > 0) {
                uint256 currentSupply = totalSupply(id);
                if (currentSupply + amounts[i] > maxSupply[id]) {
                    revert DeFiMultiToken__MaxSupplyReached(
                        id, maxSupply[id], currentSupply, amounts[i]
                    );
                }
            }
        }

        _mintBatch(to, ids, amounts, data);
    }

    // ========== 燃烧函数（重写，加上 tokenExists 检查）==========

    function burn(address accnount, uint256 id, uint256 value) public override tokenExists(id) {
        super.burn(accnount, id, value);
    }

    function burnBatch(address account, uint256[] memory ids, uint256[] memory values) public override {
        for(uint256 i = 0; i < ids.length; i++) {
            if (!_tokenExists[ids[i]]) revert DeFiMultiToken__TokenDoesNotExist(ids[i]);
        }
        super.burnBatch(account,ids, values);
    }

    // ========== URI 管理 ==========

    /// @notice 设置基础 URI（应包含 {id} 占位符，由客户端替换）
    function setBaseURI(string memory newBaseURI) external onlyOwner {
        string memory oldURI = _baseURI;
        _baseURI = newBaseURI;
        _setURI(newBaseURI); // OZ 的 ERC1155 用 _setURI 更新
        emit BaseURISet(oldURI, newBaseURI);
    }

    /// @notice 为特定 id 设置独立 URI
    function setTokenURI(uint256 id, string memory tokenURI_) external onlyOwner tokenExists(id) {
        emit URI(tokenURI_, id);
    }

    /// @notice 获取某个 id 的完整 URI
    /// @dev 如果 id 不存在，返回空字符串而不是 revert
    function uri(uint256 id) public view override tokenExists(id) returns (string memory) {
        return super.uri(id);
    }

    /// @notice 获取当前所有代币类型的数量
    function tokenCount() external view returns (uint256) {
        return _nextTokenId;
    }

    // ========== 供应量管理 ==========

    /// @notice 设置最大供应量
    function setMaxSupply(uint256 id, uint256 newMaxSupply) external onlyOwner tokenExists(id) supplyNotLocked(id) {
        uint256 currentSupply = totalSupply(id);
        if (newMaxSupply > 0 && newMaxSupply < currentSupply) {
            revert DeFiMultiToken__InvalidMaxSupply(id, currentSupply, newMaxSupply);
        }
        maxSupply[id] = newMaxSupply;
        emit MaxSupplySet(id, newMaxSupply);
    }

    /// @notice 锁定供应量（不可逆！锁死后不能再铸造）
    function lockSupply(uint256 id) external onlyOwner tokenExists(id) {
        supplyLocked[id] = true;
        emit SupplyLocked(id, totalSupply(id));
    }

    // ========== 查询辅助函数 ==========

    /// @notice 批量查询多种代币的余额
    function getBalances(address account, uint256[] memory ids) external view returns (uint256[] memory) {
        uint256[] memory balances = new uint256[](ids.length);
        for(uint256 i = 0; i < ids.length; i++) {
            balances[i] = balanceOf(account, ids[i]);
        }
        return balances;
    }

    /// @notice 获取代币信息
    function getTokenInfo(uint256 id) external view tokenExists(id) returns (uint256 totalSupply_, uint256 maxSupply_, bool isSupplyLocked_, string memory uri_) {
        return (totalSupply(id), maxSupply[id], supplyLocked[id], uri(id));
    }


    // ========== 必须重写的函数 ==========

    /// @dev ERC1155 和 ERC1155Supply 都定义了 _update，必须显式 override
    function _update(address from, address to, uint256[] memory ids, uint256[] memory values) internal override(ERC1155, ERC1155Supply) {
        super._update(from, to, ids, values);
    }
}