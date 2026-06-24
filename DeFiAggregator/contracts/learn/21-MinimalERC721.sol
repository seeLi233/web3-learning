// SPDX-License-Identifier: SEE LICENSE IN LICENSE
pragma solidity ^0.8.0;

/**
 * @title MinimalERC721
 * @notice 手写最简 ERC721（不使用 OpenZeppelin）
 *
 * 学习目的:
 *   - 理解 ERC721 的核心状态变量:
 *       _owners[tokenId] → owner      (谁拥有这个 NFT)
 *       _balances[owner] → count      (这个地址拥有几个 NFT)
 *       _tokenApprovals[tokenId] → spender  (单个 token 被授权给谁)
 *       _operatorApprovals[owner][operator] → bool  (全量授权)
 *
 *   - 理解 mint / transfer / approve 的底层逻辑
 *   - 理解 safeTransferFrom 的安全检查
 *
 * 学习完成后，再对比 OpenZeppelin 的完整实现
 */
contract MinimalERC721 {
    // ===== 事件 =====
    event Transfer(address indexed from, address indexed to, uint256 indexed tokenId);
    event Approval(address indexed owner, address indexed approved, uint256 indexed tokenId);
    event ApprovalForAll(address indexed owner, address indexed operator, bool approved);

    // ===== 存储 =====
    // 核心 mapping: tokenId → 拥有者
    mapping (uint256 => address) private _owners;

    // 辅助 mapping: 地址 → 拥有的 NFT 数量
    mapping (address => uint256) private _balances;

    // 单个 token 授权: tokenId → 被授权者
    mapping (uint256 => address) private _tokenApprovals;

    // 全量授权: owner → operator → 是否授权
    mapping (address => mapping (address => bool)) private _operatorApprovals;

    // ===== 元数据 =====
    string private _name;
    string private _symbol;

    // ===== 构造函数 =====
    constructor(string memory name_, string memory symbol_) {
        _name = name_;
        _symbol = symbol_;
    }

    // ===== 查询函数 =====
    function name() public view returns (string memory) {
        return _name;
    }

    function symbol() public view returns (string memory) {
        return _symbol;
    }

    function balanceOf(address owner) public view returns (uint256) {
        require(owner != address(0), "ERC721: zero address");
        return _balances[owner];
    }

    function ownerOf(uint256 tokenId) public view returns (address) {
        address owner = _owners[tokenId];
        require(owner != address(0), "ERC721: invalid token ID");
        return owner;
    }

    function getApproved(uint256 tokenId) public view returns (address) {
        require(_owners[tokenId] != address(0), "ERC721: invalid token ID");
        return _tokenApprovals[tokenId];
    }

    function isApprovedForAll(address owner, address operator) public view returns (bool) {
        return _operatorApprovals[owner][operator];
    }

    // ===== 授权函数 =====
    function approve(address to, uint256 tokenId) public {
        address owner =  _owners[tokenId];
        // 只有 owner 或 被全量授权的 operator 才能 approve
        require(msg.sender == owner || isApprovedForAll(owner, msg.sender), "ERC721: not authorized");
        _tokenApprovals[tokenId] = to;
        emit Approval(owner, to, tokenId);
    }

    function setApproveForAll(address operator, bool approved) public {
        require(operator != msg.sender, "ERC721: approve to caller");
        _operatorApprovals[msg.sender][operator] = approved;
        emit ApprovalForAll(msg.sender, operator, approved);
    }

    // ===== 转账函数 =====
    function transferFrom(address from, address to, uint256 tokenId) public {
        // 权限检查：调用者必须是 owner 或被授权的
        require(_isApprovedOrOwner(msg.sender, tokenId), "ERC721: not authorized");
        require(ownerOf(tokenId) == from, "ERC721: not owner");
        require(to != address(0), "ERC721: transfer to zero address");

        // 转账前清楚授权
        delete _tokenApprovals[tokenId];

        // 更新状态
        _balances[from]--;
        _balances[to]++;
        _owners[tokenId] = to;

        emit Transfer(from, to, tokenId);
    }

    function safeTransferFrom(address from, address to, uint256 tokenId) public {
        safeTransferFrom(from, to, tokenId, "");
    }

    function safeTransferFrom(address from, address to, uint256 tokenId, bytes memory data) public {
        transferFrom(from, to, tokenId);

        // ★ 安全检查: 如果接收方是合约，必须能响应 onERC721Received
        require(
            _checkOnERC721Received(from, to, tokenId, data),
            "ERC721: transfer to non ERC721Receiver implementer"
        );
    }

    // ===== 内部函数 =====

    // 铸造 NFT: from = address(0) 表示"从无到有"
    function _mint(address to, uint256 tokenId) internal {
        require(to != address(0), "ERC721: mint to zero address");
        require(_owners[tokenId] == address(0), "ERC721: token already minted");

        _balances[to]++;
        _owners[tokenId] = to;

        // ⚠️ 铸造时 from = address(0)
        emit Transfer(address(0), to, tokenId);
    }

    // 燃烧 NFT: to = address(0) 表示"从有到无"
    function _burn(uint256 tokenId) internal {
        address owner = ownerOf(tokenId);

        delete _tokenApprovals[tokenId];
        _balances[owner]--;
        delete _owners[tokenId];

        emit Transfer(owner, address(0), tokenId);
    }

    // 权限检查: msg.sender 是否是 owner/approved/operator
    function _isApprovedOrOwner(address spender, uint256 tokenId) internal view returns (bool) {
        address owner = ownerOf(tokenId);
        return (spender == owner || getApproved(tokenId) == spender || isApprovedForAll(owner, spender));
    }

    // 检查接收方合约是否支持 ERC721
    function _checkOnERC721Received(address from, address to, uint256 tokenId, bytes memory data) private returns (bool) {
        // EOA 地址直接通过 (普通钱包不需要实现 IERC721Receiver)
        if (_isContract(to)) {
            try IERC721Receiver(to).onERC721Received(msg.sender, from, tokenId, data) returns (bytes4 retval) {
                return retval == IERC721Receiver.onERC721Received.selector;
            } catch  {
                return false;
            }
        }
        return true;
    }

    function _isContract(address account) private view returns (bool) {
        // Solidity 0.8+ 可以用 account.code.lenth > 0
        return account.code.length >0;
    }
}

// 接收方接口
interface IERC721Receiver {
    function onERC721Received(address operator, address from, uint256 tokenId, bytes calldata data) external returns (bytes4);
}