// SPDX-License-Identifier: SEE LICENSE IN LICENSE
pragma solidity ^0.8.0;

/**
 * @title URIStorageDemo
 * @notice 演示两种 URI 管理方式 + Base64 链上元数据
 *
 * 核心概念:
 *   1. baseURI: 所有 token 的公共基础路径，省钱
 *   2. tokenURI: 每个 token 的独立路径，灵活
 *   3. 链上 Base64 JSON: 完全去中心化的元数据，适合 SVG NFT
 */
contract URIStorageDemo {
    string private _name;
    string private _symbol;

    // 方式1: 公共 baseURI + tokenId 拼接
    string private _baseURI;    // 例如: "ipfs://QmWJ8...abc/"
    // tokenURI(42) = baseURI + "42.json" = "ipfs://QmWJ8...abc/42.json"

    // 方式2: 每个 token 独立的 URI（可覆盖 baseURI）
    mapping (uint256 => string) private _tokenURIs;

    // 追踪 tokenId
    uint256 private _nextTokenId;
    mapping (uint256 => address) private _owners;

    event  Minted(address indexed to, uint256 indexed tokenId, string uri);

    constructor(string memory name_, string memory symbol_) {
        _name = name_;
        _symbol = symbol_;
    }

    // ===== baseURI 管理 =====
    function setBaseURI(string memory baseURI_) public {
        _baseURI = baseURI_;
    }

    function baseURI() public view returns (string memory) {
        return _baseURI;
    }

    // ===== 铸造 =====
    // 方式 A: 使用 baseURI + tokenId（适合批量铸造，所有 URI 格式一致）
    function mintWithBaseURI(address to) public returns (uint256) {
        uint256 tokenId = _nextTokenId;
        _nextTokenId++;
        _owners[tokenId] = to;
        // tokenURI 自动 = baseURI + tokenId
        emit Minted(to,tokenId, tokenURI(tokenId));
        return tokenId;
    }

    // 方式 B: 每个 token 单独设置 URI（适合每个 NFT 不同的元数据）
    function mintWithCustomURI(address to, string memory customURI) public returns (uint256) {
        uint256 tokenId = _nextTokenId;
        _nextTokenId++;
        _owners[tokenId] = to;
        _tokenURIs[tokenId] = customURI;
        emit Minted(to, tokenId, customURI);
        return tokenId;
    }

    // ===== tokenURI 查询 =====
    function tokenURI(uint256 tokenId) public view returns (string memory) {
        // 优先级：单独设置的 URI > baseURI + tokenId
        string memory customURI = _tokenURIs[tokenId];
        if(bytes(customURI).length > 0) {
            return customURI;
        }

        // 如果设置了 baseURI，拼接 tokenId
        if (bytes(_baseURI).length > 0) {
            return string(abi.encodePacked(_baseURI, _toString(tokenId)));
        }

        return "";
    }

    // ===== 链上 Base64 元数据（完全去中心化）=====
    // 生成 Base64 编码的 JSON 元数据，直接写在链上
    function tokenURIOnChain(uint256 tokenId) public view returns (string memory) {
        // 构造 JSON 元数据
        bytes memory json = abi.encodePacked(
            '{"name":"', _name, ' #', _toString(tokenId), '",',
            '"description":"A fully on-chain NFT",',
            '"image":"data:image/svg+xml;base64,', generateSVG(tokenId), '",',
            '"attributes":[{"trait_type":"Rarity","value":"', _toString((tokenId % 100) + 1), '"}]',
            '}'
        );
        // 转 Base64
        return string(abi.encodePacked("data:application/json;base64,", _base64Encode(json)));
    }

    // 生成简单的 SVG 图片（链上）
    function generateSVG(uint256 tokenId) public pure returns (string memory) {
        // Base64 编码的 SVG
        return string(abi.encodePacked("PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIyMDAiIGhlaWdodD0iMjAwIj"
            "48cmVjdCB3aWR0aD0iMjAwIiBoZWlnaHQ9IjIwMCIgZmlsbD0iIzMzMyIvP"
            "jx0ZXh0IHg9IjEwMCIgeT0iMTAwIiBmb250LXNpemU9IjQwIiBmaWxsPSJ3aGl0ZSIgdGV4dC1hbmNob3I9Im1pZG"
            "RsZSIgZHk9Ii4zZW0iPg=="));
    }

    // ===== 工具函数 =====
    // uint256 → string
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

    // Base64 编码（简化版，只演示概念。实际项目建议用 OZ 的 Base64 库）
    // 正式项目中 import "@openzeppelin/contracts/utils/Base64.sol";
    function _base64Encode(bytes memory data) internal pure returns (string memory) {
        // 简化处理，实际使用 OpenZeppelin 的 Base64.encode(data)
        // 这里只是为了展示链上元数据的思路
        return "BASE64_PLACEHOLDER";
    }
}