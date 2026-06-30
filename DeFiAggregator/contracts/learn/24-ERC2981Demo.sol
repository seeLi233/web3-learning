// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

// =============================================
// 24-ERC2981RoyaltyDemo.sol — Day 11 学习文件
// 知识点: ERC2981 版税 + 链上 SVG + Base64 编码
// =============================================

// 学习目标:
// 1. 手写最简 ERC2981 实现 (理解底层)
// 2. 链上 SVG 生成 + Base64 编码
// 3. 验证 ERC165 接口支持

import "@openzeppelin/contracts/utils/Base64.sol";
import "@openzeppelin/contracts/utils/Strings.sol";

// =============================================
// 合约 1: MinimalERC2981 — 最简 ERC2981 实现
// 目的: 理解 ERC2981 的底层存储和计算逻辑
// =============================================
contract MinimalERC2981 {
    // ===== 数据结构 =====
    // 注意: address(160位) + uint96(96位) = 恰好填满一个 slot
    struct RoyaltyInfo {
        address receiver;
        uint96 royaltyFraction;     // basis points, 如 500 = 5%
    }

    // ===== 存储 =====
    // 每个 tokenId 可以设置独立的版税信息
    mapping (uint256 => RoyaltyInfo) private _tokenRoyaltyInfo;

    // 默认版税信息 (适用于未单独设置的 token)
    RoyaltyInfo private _defaultRoyaltyInfo;

    // ===== 常量 =====
    // 费率分母: 10000 basis points = 100%
    uint256 private constant _FEE_DENOMINATOR = 10000;

    // ===== 错误 =====
    error ERC2981__InvalidRoyaltyFraction();
    error ERC2981__ZeroAddress();

    // ===== 事件 =====
    event DefaultRoyaltySet(address indexed receiver, uint96 royaltyFraction);
    event TokenRoyaltySet(uint256 indexed tokenId, address indexed receiver, uint96 royaltyFraction);

    // ===== 核心查询函数 =====
    /**
     * @notice ERC2981 标准接口 — 查询版税信息
     * @param tokenId  要查询的 NFT ID
     * @param salePrice  假设的销售价格 (同单位)
     * @return receiver 版税接收者
     * @return royaltyAmount 版税金额
     *
     * 查询优先级: token 独立设置 > 默认设置 > (address(0), 0)
     */
    function royaltyInfo(uint256 tokenId, uint256 salePrice) public view returns (address receiver, uint256 royaltyAmount) {
        // 第一步：查这个 token 有没有独立版税设置
        RoyaltyInfo memory royalty = _tokenRoyaltyInfo[tokenId];

        // 第二步：没有独立设置 -> 用默认版税
        if (royalty.receiver == address(0)) {
            royalty = _defaultRoyaltyInfo;
        }

        // 第三步：计算版税金额
        // 公式：amount = price x fraction / 10000
        // 注意: 先乘后除，避免精度损失
        royaltyAmount = (salePrice * uint256(royalty.royaltyFraction)) / _FEE_DENOMINATOR;

        return (royalty.receiver, royaltyAmount);
    }

    // ===== 内部设置函数 =====
    /**
     * @notice 设置默认版税 (所有无独立设置的 token 都用这个)
     * @param receiver 版税接收者
     * @param royaltyFraction 费率 (basis points)
     */
    function _setDefaultRoyalty(address receiver, uint96 royaltyFraction) internal {
        if (receiver == address(0)) revert ERC2981__ZeroAddress();
        if (royaltyFraction > _FEE_DENOMINATOR) revert ERC2981__InvalidRoyaltyFraction();
        // 不能超过 100% (10000 bps)

        _defaultRoyaltyInfo = RoyaltyInfo(receiver, royaltyFraction);
        emit DefaultRoyaltySet(receiver, royaltyFraction);
    }

    /**
     * @notice 设置某个 token 的独立版税
     */
    function _setTokenRoyalty(uint256 tokenId, address receiver, uint96 royaltyFraction) internal {
        if (receiver == address(0)) revert ERC2981__ZeroAddress();
        if (royaltyFraction > _FEE_DENOMINATOR) revert ERC2981__InvalidRoyaltyFraction();
        // 不能超过 100% (10000 bps)

        _tokenRoyaltyInfo[tokenId] = RoyaltyInfo(receiver, royaltyFraction);
        emit TokenRoyaltySet(tokenId, receiver, royaltyFraction);
    }

    /**
     * @notice 重置某个 token 的版税为默认值
     */
    function _resetTokenRoyalty(uint256 tokenId) internal {
        delete _tokenRoyaltyInfo[tokenId];
    }
}

// =============================================
// 合约 2: OnChainSVG — 链上 SVG 生成合约
// 目的: 学习如何用 Solidity 动态生成 SVG 图片
// =============================================
contract OnChainSVG {
    using Strings for uint256;

    /**
     * @notice 动态生成 SVG 图片 (不包含 Base64 编码)
     * @param tokenId  NFT 的唯一 ID
     *
     * SVG 基础语法:
     *  <svg>  — 画布
     *  <rect> — 矩形（背景）
     *  <text> — 文字
     *  <circle> — 圆形
     *
     * 用 abi.encodePacked 拼接字符串生成 SVG
     */
    function generateSVG(uint256 tokenId) public pure returns (string memory) {
        // 颜色由 tokenId 决定 — 不同 tokenId 生成不同颜色
        // 取 tokenId 的低 24 位作为 RGB
        string memory color = _toHexColor(tokenId);

        return string(
            abi.encodePacked(
                '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 350 350">',
                // 背景矩形
                '<rect width="350" height="350" fill="',
                color,
                '"/>',
                // 白色半透明圆
                '<circle cx="175" cy="150" r="100" fill="rgba(255,255,255,0.2)"/>',
                '#',
                tokenId.toString(),
                '</text>',
                // 底部文字
                '<text x="175" y="300" font-family="Arial" font-size="20"',
                ' fill="rgba(255,255,255,0.7)" text-anchor="middle">',
                'On-Chain NFT',
                '</text>',
                '</svg>'
            )
        );
    }

    /**
     * @notice 获取完整的 tokenURI (data URI 格式)
     *
     * 数据流水线:
     *   SVG 字符串 → Base64(SVG) → JSON → Base64(JSON) → data URI
     */
    function tokenURI(uint256 tokenId) public view virtual returns (string memory) {
        // 第 1 步: 生成 SVG
        string memory svg = generateSVG(tokenId);

        // 第 2 步: SVG → Base64
        string memory base64SVG = Base64.encode(bytes(svg));

        // 第 3 步: 构建 JSON 元数据
        string memory json = string(
            abi.encodePacked(
                '{"name":"On-Chain NFT #',
                tokenId.toString(),
                '","description":"A fully on-chain NFT with SVG art.","image":"data:image/svg+xml;base64,',
                base64SVG,
                '","attributes":[{"trait_type":"Color","value":"',
                _toHexColor(tokenId),
                '"}]}'
            )
        );

        // 第 4 步: JSON → Base64
        string memory base64JSON = Base64.encode(bytes(json));

        // 第 5 步: 拼接 data URI
        return string(abi.encodePacked("data:application/json;base64,", base64JSON));
    }

    /**
     * @notice 从 uint256 生成十六进制颜色字符串 (如 "#1a2b3c")
     * 取低 24 位作为 RGB 颜色
     */
    function _toHexColor(uint256 value) internal pure returns (string memory) {
        // 取前 24 位作为 RGB
        bytes memory hexChars = "0123456789abcdef";
        bytes memory color = new bytes(7);
        color[0] = '#';

        // 取低 24 位: 每 8 位是一个颜色通道
        uint256 r = (value >> 16) & 0xFF;   // 红色 (bits 16-23)
        uint256 g = (value >> 8) & 0xFF;    // 绿色 (bits 8-15)
        uint256 b = value & 0xFF;           // 蓝色 (bits 0-7)

        color[1] = hexChars[r >> 4];
        color[2] = hexChars[r & 0xF];
        color[3] = hexChars[g >> 4];
        color[4] = hexChars[g & 0xF];
        color[5] = hexChars[b >> 4];
        color[6] = hexChars[b & 0xF];

        return string(color);
    }
}

// =============================================
// 合约 3: RoyaltyNFTPure — 纯教学版（手动实现一切）
// 目的: 理解版税 + SVG + Base64 的完整组合
// 注意: 这个合约教学用，不依赖 ERC721
// =============================================
contract RoyaltyDemo is MinimalERC2981, OnChainSVG {
    // 这个合约演示了多个合约的组合
    // MinimalERC2981 提供版税支持
    // OnChainSVG 提供链上 SVG
    
    address public owner;
    uint256 private _nextTokenId;

    constructor(uint96 royaltyBps) {
        owner = msg.sender;
        // 设置默认版税: 接收者 = owner, 费率 = royaltyBps
        if (royaltyBps > 0) {
            _setDefaultRoyalty(owner, royaltyBps);
        }
    }

    /// @notice 铸造一个新 NFT
    function mint() public returns (uint256) {
        uint256 tokenId = _nextTokenId;
        _nextTokenId++;
        // 实际项目中这里会 _safeMint(to, tokenId)
        return tokenId;
    }

    /// @notice 总铸造量
    function totalMinted() public view returns (uint256) {
        return _nextTokenId;
    }

    /// @notice 更新版税率
    function updateRoyalty(uint96 royaltyBps) public {
        require(msg.sender == owner, "Not owner");
        _setDefaultRoyalty(owner, royaltyBps);
    }
}