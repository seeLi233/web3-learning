// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

// =============================================
// MyNFT.sol — 完整 ERC721 NFT 合约
// 基于 OpenZeppelin 实现
//
// 继承链:
//   ERC721 → ERC721URIStorage → ERC721Enumerable → ERC721Burnable → Ownable
//
// 功能清单:
//   ✅ 铸造 (safeMint) — owner 可以给任何人铸造
//   ✅ 燃烧 (burn) — token 拥有者可以销毁
//   ✅ 可枚举 (Enumerable) — 可以遍历全部 tokenId
//   ✅ URI 存储 (URIStorage) — 每个 NFT 独立的元数据
//   ✅ 权限控制 (Ownable) — onlyOwner 铸造
//   ✅ baseURI + tokenURI 双层管理
//   ✅ 安全转账 (safeTransferFrom) — 防止锁死
//
// 版本: v1.0
// =============================================

import "@openzeppelin/contracts/token/ERC721/ERC721.sol";
import "@openzeppelin/contracts/token/ERC721/extensions/ERC721URIStorage.sol";
import "@openzeppelin/contracts/token/ERC721/extensions/ERC721Enumerable.sol";
import "@openzeppelin/contracts/token/ERC721/extensions/ERC721Burnable.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

/**
 * @title MyNFT
 * @notice 一个功能完整的 NFT 合约
 *
 * 🏗️ 继承链详解:
 *   ERC721 (基础)
 *     ├── ERC721URIStorage (扩展: 每个 tokenId 独立 URI)
 *     ├── ERC721Enumerable (扩展: 可遍历所有 tokenId)
 *     └── ERC721Burnable   (扩展: 可燃烧销毁)
 *   Ownable (权限控制)
 *
 * ⚠️ 注意继承顺序:
 *   MyNFT is ERC721, ERC721URIStorage, ERC721Enumerable, ERC721Burnable, Ownable
 *   这个顺序很重要——ERC721 必须在最前，URIStorage 在 Enumerable 之前
 */
contract MyNFT is ERC721, ERC721URIStorage, ERC721Enumerable, ERC721Burnable, Ownable {
    // ===== 错误定义 =====
    error MyNFT__MaxSupplyReached();
    error MyNFT__TokenNotFound();
    error MyNFT__NotTokenOwner();
    error MyNFT__ZeroAddress();

    // ===== 状态变量 =====
    uint256 private _nextTokenId;       // 下一个可用的 tokenId (自增)
    uint256 public constant MAX_SUPPLY= 10000; // 最大供应量
    string private _baseTokenURI;       // 基础 URI

    // ===== 事件 =====
    event NFTMinted(address indexed to, uint256 indexed tokenId, string uri);
    event BaseURIUpdated(string newBaseURI);

    // ===== 构造函数 =====
    /**
     * @param name_ NFT 合集名称，如 "My First NFT"
     * @param symbol_ NFT 合集简称，如 "MFN"
     * @param baseURI_ 初始 baseURI，如 "ipfs://QmXxx...abc/"
     */
    constructor(string memory name_, string memory symbol_, string memory baseURI_) ERC721(name_, symbol_) Ownable(msg.sender) {
        _baseTokenURI = baseURI_;
    }

    // ===== 铸造函数 =====
    /**
     * @notice 安全铸造一个 NFT 给指定地址
     * @param to 接收者地址
     * @param uri 这个 NFT 的元数据 URI（如 "ipfs://Qm.../42.json"）
     *            如果为空字符串，则使用 baseURI + tokenId
     * @return tokenId 新铸造的 NFT 的唯一 ID
     *
     * 权限: onlyOwner
     */
    function safeMint(address to, string memory uri) public onlyOwner returns (uint256) {
        if (_nextTokenId >= MAX_SUPPLY) {
            revert MyNFT__MaxSupplyReached();
        }
        if (to == address(0)) {
            revert MyNFT__ZeroAddress();
        }

        uint256 tokenId = _nextTokenId;
        _nextTokenId++;

        // 铸造 (ERC721 内部函数, from = address(0) 表示铸造)
        _safeMint(to, tokenId);

        // 设置 URI
        if (bytes(uri).length > 0) {
            _setTokenURI(tokenId, uri); // 用指定的 URI
        }
        // 如果 uri 为空, tokenURI() 自动 fallback 到 baseURI + tokenId

        emit NFTMinted(to, tokenId, tokenURI(tokenId));
        return tokenId;
    }

    /**
     * @notice 批量铸造多个 NFT（每个 token 可以有不同的 URI）
     * @param tos 接收者地址列表
     * @param uris 每个 token 的 URI 列表
     *
     * ⚠️ 注意 gas: 批量操作越多越贵
     */
    function batchMint(address[] calldata tos, string[] calldata uris) public onlyOwner {
        require(tos.length == uris.length, "Length mismatch");
        for (uint256 i = 0; i < tos.length; i++) {
            safeMint(tos[i], uris[i]);
        }
    }

    // ===== URI 管理 =====
    /**
     * @notice 更新 baseURI（所有使用 baseURI 的 token 都会受到影响）
     * 权限: onlyOwner
     */
    function setBaseURI(string memory baseURI_) public onlyOwner {
        _baseTokenURI = baseURI_;
        emit BaseURIUpdated(baseURI_);
    }

    /**
     * @notice 更新某个 token 的独立 URI
     * 权限: onlyOwner
     */
    function setTokenURI(uint256 tokenId, string memory uri) public onlyOwner {
        if (_ownerOf(tokenId) == address(0)) {
            revert MyNFT__TokenNotFound();
        }
        _setTokenURI(tokenId, uri);
    }

    // ===== 查询函数 =====

    /// @notice 返回所有已铸造的 tokenId 列表（由 Enumerable 扩展提供）
    function getAllTokens() public view returns (uint256[] memory) {
        uint256 total = totalSupply();
        uint256[] memory tokens = new uint256[](total);
        for(uint256 i = 0; i < total; i++) {
            tokens[i] = tokenByIndex(i);  // ERC721Enumerable 提供
        }
        return tokens;
    }

    /// @notice 返回某个地址拥有的所有 tokenId
    function getTokensOfOwner(address owner) public view returns (uint256[] memory) {
        uint256 count = balanceOf(owner);
        uint256[] memory tokens = new uint256[](count);
        for (uint256 i = 0; i < count; i++) {
            tokens[i] = tokenOfOwnerByIndex(owner, i);
        }
        return tokens;
    }

    /// @notice 总供应量
    function totalMinted() public view returns (uint256) {
        return _nextTokenId;    // 已经铸造到第几个了
    }

    // ===== 必须重写的函数（多重继承冲突解析）=====

    /**
     * @notice 当 token 转移/铸造/燃烧时，更新 Enumerable 的索引
     *
     * ⚠️ Solidity 要求显式 override 因为:
     *   ERC721._update 和 ERC721Enumerable._update 都有实现
     *   必须告诉编译器用哪个
     */
    function _update(address to, uint256 tokenId, address auth) internal override(ERC721, ERC721Enumerable) returns (address) {
        return super._update(to, tokenId, auth);
    }

    /**
     * @notice 查询某个 tokenId 是否被燃烧
     * 内部用来计算 totalSupply()
     */
    function _increaseBalance(address account, uint128 value) internal override(ERC721, ERC721Enumerable) {
        super._increaseBalance(account, value);
    }

    /**
     * @notice tokenURI 的 fallback 逻辑:
     *   1. 如果有单独设置的 _tokenURIs[tokenId] → 用它
     *   2. 如果设置了 baseURI → 用 baseURI + tokenId
     *   3. 都没有 → 返回空字符串
     */
    function tokenURI(uint256 tokenId) public view override(ERC721, ERC721URIStorage) returns (string memory) {
        if (_ownerOf(tokenId) == address(0)) {
            revert MyNFT__TokenNotFound();
        }

        // 先检查有没有单独设置的 URI
        string memory customURI = super.tokenURI(tokenId);

        // 如果单独设置的 URI 不为空，直接返回
        if (bytes(customURI).length >0) {
            return customURI;
        }

        // Fallback: baseURI + tokenId
        if (bytes(_baseTokenURI).length > 0) {
            return string(abi.encodePacked(_baseTokenURI, _toString(tokenId)));
        }

        return "";
    }

    /**
     * @notice 检查合约支持哪些接口（ERC165 标准）
     * 面试常问: 合约如何告诉外界"我支持哪些功能"？
     */
    function supportsInterface(bytes4 interfaceId) public view override(ERC721, ERC721URIStorage, ERC721Enumerable) returns (bool) {
        return super.supportsInterface(interfaceId);
    }

    // ===== 工具函数 =====
    function _toString(uint256 value) internal pure returns (string memory) {
        if (value == 0) return "0";
        uint256 temp = value;
        uint256 digits;
        while (temp != 0) {
            digits++;
            temp /= 10;
        }
        bytes memory buffer = new bytes(digits);
        while (value != 0) {
            digits -= 1;
            buffer[digits] = bytes1(uint8(48 + uint256(value % 10)));
            value /= 10;
        }
        return string(buffer);
    }
}