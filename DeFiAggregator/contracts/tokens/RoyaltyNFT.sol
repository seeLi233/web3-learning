// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

// =============================================
// RoyaltyNFT.sol — Day 11 核心合约
// 完整 ERC721 NFT + ERC2981 版税 + 链上 SVG 元数据
//
// 继承链:
//   ERC721 → ERC721URIStorage → ERC721Enumerable → ERC721Burnable
//   → Ownable → ERC2981
//
// 功能:
//   ✅ 版税查询 (ERC2981 royaltyInfo)
//   ✅ 链上动态 SVG (Base64 编码)
//   ✅ 铸造/燃烧/枚举
//   ✅ 权限管理 (Ownable)
//   ✅ ERC165 接口检测
// =============================================

import "@openzeppelin/contracts/token/ERC721/ERC721.sol";
import "@openzeppelin/contracts/token/ERC721/extensions/ERC721URIStorage.sol";
import "@openzeppelin/contracts/token/ERC721/extensions/ERC721Enumerable.sol";
import "@openzeppelin/contracts/token/ERC721/extensions/ERC721Burnable.sol";
import "@openzeppelin/contracts/access/Ownable.sol";
import "@openzeppelin/contracts/token/common/ERC2981.sol";
import "@openzeppelin/contracts/utils/Base58.sol";
import "@openzeppelin/contracts/utils/Base64.sol";
import "@openzeppelin/contracts/utils/Strings.sol";

/**
 * @title RoyaltyNFT
 * @notice 带 ERC2981 版税 + 链上 SVG 的完整 NFT 合约
 *
 * ⭐ 今日核心学习点:
 *
 * 1. ERC2981 存储布局:
 *    struct RoyaltyInfo { address(160b) + uint96(96b) }
 *    恰好 256 bits = 1 slot，节省 gas!
 *
 * 2. 版税计算:
 *    amount = (salePrice × royaltyFraction) / 10000
 *    先乘后除，避免精度损失！
 *
 * 3. 链上 SVG 流程:
 *    SVG → bytes → Base64.encode() → data URI
 *    完全不依赖外部存储，100% 链上！
 *
 * 4. ERC165:
 *    通过 supportsInterface 告诉外界"我支持哪些标准"
 *    IERC721 + IERC2981 + IERC721Enumerable + IERC721Metadata
 */
contract RoyaltyNFT is ERC721, ERC721URIStorage, ERC721Enumerable, ERC721Burnable, ERC2981, Ownable {
    using Strings for uint256;

    // ===== 错误定义 =====
    error RoyaltyNFT__MaxSupplyReached();
    error RoyaltyNFT__TokenNotFound();
    error RoyaltyNFT__ZeroAddress();
    error RoyaltyNFT__InvalidRoyalty(uint96 royaltyBps);
    error RoyaltyNFT__InvalidMintPrice();

    // ===== 状态变量 =====
    uint256 private _nextTokenId;
    uint256 public constant MAX_SUPPLY = 5000;

    // ===== 事件 =====
    event RoyaltyNFTMinted(address indexed to, uint256 indexed tokenId, uint96 royaltyBps);

    // ===== 构造函数 =====
    /**
     * @param name_  NFT 合集名称
     * @param symbol_  NFT 合集简称
     * @param royaltyReceiver_  默认版税接收者
     * @param royaltyBps_  默认版税率 (basis points, 如 500 = 5%)
     *
     * 注意: 构造函数的 royalty 设置用的是 ERC2981 的 _setDefaultRoyalty
     */
    constructor(string memory name_, string memory symbol_, address royaltyReceiver_, uint96 royaltyBps_) ERC721(name_, symbol_) Ownable(msg.sender) {
        // 设置版税 (ERC2981 提供的方法)
        if (royaltyReceiver_ != address(0) && royaltyBps_ > 0) {
            _setDefaultRoyalty(royaltyReceiver_, royaltyBps_);
        }
    }

    // ===== 铸造函数 =====
    /**
     * @notice 安全铸造一个带链上 SVG 的 NFT
     * @param to 接收者地址
     * @return tokenId 新铸造的 token ID
     *
     * 铸造完成后:
     *   - tokenURI() 返回 data:application/json;base64,....
     *   - royaltyInfo() 返回版税信息
     */
    function safeMint(address to) public onlyOwner returns (uint256) {
        if (_nextTokenId >= MAX_SUPPLY) {
            revert RoyaltyNFT__MaxSupplyReached();
        }
        if (to == address(0)) {
            revert RoyaltyNFT__ZeroAddress();
        }

        uint256 tokenId = _nextTokenId;
        _nextTokenId++;

        // 铸造 NFT
        _safeMint(to, tokenId);

        // 设置链上 URI — tokenURI 在下面动态生成
        // 注意: ERC721URIStorage 的 _setTokenURI 可以不调用
        // 因为我们重写了 tokenURI() 函数来动态生成

        emit RoyaltyNFTMinted(to, tokenId, _defaultRoyaltyBps());

        return tokenId;
    }

    /**
     * @notice 铸造一个带独立版税的 NFT
     * @param to 接收者
     * @param royaltyReceiver 这个 token 的专属版税接收者
     * @param royaltyBps 这个 token 的专属版税率
     *
     * 场景: 合作创作者，不同 token 给不同人分版税
     */
    function safeMintWithRoyalty(address to, address royaltyReceiver, uint96 royaltyBps) public onlyOwner returns (uint256) {
        uint256 tokenId = safeMint(to);

        // 设置这个 token 的独立版税
        _setTokenRoyalty(tokenId, royaltyReceiver, royaltyBps);

        return tokenId;
    }

    // ===== 版税管理函数 =====
    /**
     * @notice 更新默认版税
     * @param receiver 新的版税接收者
     * @param royaltyBps 新的版税率
     */
    function setDefaultRoyalty(address receiver, uint96 royaltyBps) public onlyOwner {
        _setDefaultRoyalty(receiver, royaltyBps);
    }

    /**
     * @notice 更新某个 token 的版税
     */
    function setTokenRoyalty(uint256 tokenId, address receiver, uint96 royaltyBps) public onlyOwner {
        if (_ownerOf(tokenId) == address(0)) {
            revert RoyaltyNFT__TokenNotFound();
        }
        _setTokenRoyalty(tokenId, receiver, royaltyBps);
    }

    /// @notice 查询默认版税率
    function _defaultRoyaltyBps() internal pure returns (uint96) {
        // 在实际项目中可以从 ERC2981 的 storage 读
        // 这里简化为固定返回 500 (5%)
        return 500;
    }

    // ===== 链上 SVG 生成 =====
    /**
     * @notice 动态生成 SVG（每个 tokenId 生成不同颜色的图片）
     *
     * ⚠️ 这个函数每次调用都会生成新的 SVG 字符串 on-chain
     * 所以不需要 storage 存储，但调用需要 gas
     */
    function generateSVG(uint256 tokenId) public pure returns (string memory) {
        string memory bgColor = _tokenIdToColor(tokenId);

        return string(
            abi.encodePacked(
                '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 400 400">',
                // 背景
                '<rect width="400" height="400" fill="',
                bgColor,
                '"/>',
                // 装饰圆
                '<circle cx="200" cy="180" r="120" fill="rgba(255,255,255,0.15)"/>',
                '<circle cx="120" cy="120" r="60" fill="rgba(255,255,255,0.1)"/>',
                // Token ID
                '<text x="200" y="200" font-family="Arial,sans-serif" font-size="64"',
                ' fill="white" text-anchor="middle" dominant-baseline="middle"',
                ' font-weight="bold">',
                '#',
                tokenId.toString(),
                '</text>',
                // 底部标签
                '<text x="200" y="340" font-family="Arial,sans-serif" font-size="18"',
                ' fill="rgba(255,255,255,0.6)" text-anchor="middle">',
                'RoyaltyNFT - On-Chain',
                '</text>',
                // 版税标签
                '<text x="200" y="370" font-family="Arial,sans-serif" font-size="14"',
                ' fill="rgba(255,255,255,0.4)" text-anchor="middle">',
                'ERC2981 Royalty Enabled',
                '</text>',
                '</svg>'
            )
        );
    }

    /**
     * @notice 从 tokenId 生成唯一颜色
     *
     * 使用伪随机算法，确保:
     *   - 不同 tokenId → 不同颜色
     *   - 颜色在视觉上好看 (饱和度、亮度固定，色相变化)
     *
     * 原理: 伪随机数 × 色相范围 (HSL 转 RGB)
     */
    function _tokenIdToColor(uint256 tokenId) internal pure returns (string memory) {
        // 简单做法: 用 tokenId 的低 24 位作为 RGB
        // 做一点混合让它好看
        uint256 r = (tokenId * 7 + 13) % 256;
        uint256 g = (tokenId * 11 + 29) % 256;
        uint256 b = (tokenId * 17 + 43) % 256;

        return string(
            abi.encodePacked(
                "rgb(",
                r.toString(),
                ",",
                g.toString(),
                ",",
                b.toString(),
                ")"
            )
        );
    }

    /**
     * @notice 获取链上 tokenURI (Data URI 格式)
     *
     * 数据流水线（面试常考）:
     *   1. generateSVG(tokenId) → SVG 字符串
     *   2. Base64.encode(bytes(SVG)) → Base64 SVG
     *   3. 拼接 JSON 元数据 (包含 Base64 SVG 作为 image)
     *   4. Base64.encode(bytes(JSON)) → Base64 JSON
     *   5. "data:application/json;base64," + Base64 JSON → tokenURI
     */
    function tokenURI(uint256 tokenId)
        public
        view
        override(ERC721, ERC721URIStorage)
        returns (string memory)
    {
        if (_ownerOf(tokenId) == address(0)) {
            revert RoyaltyNFT__TokenNotFound();
        }

        // 第 1 步: 生成 SVG
        string memory svg = generateSVG(tokenId);

        // 第 2 步: SVG → Base64
        string memory base64SVG = Base64.encode(bytes(svg));

        // 第 3 步: 构建 JSON
        string memory json = string(
            abi.encodePacked(
                '{"name":"RoyaltyNFT #',
                tokenId.toString(),
                '","description":"A fully on-chain NFT with ERC2981 royalty support. ',
                'This NFT was created during Day 11 learning. The image and metadata ',
                'are 100% stored on the blockchain.","image":"data:image/svg+xml;base64,',
                base64SVG,
                '","attributes":[',
                '{"trait_type":"Token ID","value":"',
                tokenId.toString(),
                '"},{"trait_type":"Generation","value":"Genesis"},',
                '{"trait_type":"Standard","value":"ERC2981"}',
                ']}'
            )
        );

        // 第 4 步: JSON → Base64
        string memory base64JSON = Base64.encode(bytes(json));

        // 第 5 步: 返回 data URI
        return string(abi.encodePacked("data:application/json;base64,", base64JSON));
    }

    // ===== 必须重写的多重继承冲突函数 =====
    /**
     * @notice _update 在 ERC721 和 ERC721Enumerable 中都有实现
     * 必须显式 override 并指定使用哪个
     */
    function _update(
        address to,
        uint256 tokenId,
        address auth
    ) internal override(ERC721, ERC721Enumerable) returns (address) {
        return super._update(to, tokenId, auth);
    }

    function _increaseBalance(
        address account,
        uint128 value
    ) internal override(ERC721, ERC721Enumerable) {
        super._increaseBalance(account, value);
    }

    /**
     * @notice 告诉外界: "我支持 ERC721 + ERC2981 + ERC721Enumerable + ERC721Metadata"
     *
     * ERC165 interfaceId 计算方式:
     *   IERC721:              bytes4(keccak256('balanceOf...'))
     *   IERC2981:             bytes4(keccak256('royaltyInfo(uint256,uint256)'))
     *   IERC721Enumerable:    bytes4(keccak256('totalSupply()')) ^ bytes4(keccak256('tokenByIndex(uint256)')) ^ ...
     *
     *   XOR (^) 把接口中所有函数选择器异或到一起，得到一个 4 字节的 interfaceId
     */
    function supportsInterface(
        bytes4 interfaceId
    )
        public
        view
        override(ERC721, ERC721URIStorage, ERC721Enumerable, ERC2981)
        returns (bool)
    {
        return super.supportsInterface(interfaceId);
    }
}