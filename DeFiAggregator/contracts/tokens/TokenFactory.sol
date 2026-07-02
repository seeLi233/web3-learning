// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

// ===== 导入 OpenZeppelin =====
// Ownable: 提供 onlyOwner 修饰符，工厂的管理权限
// ReentrancyGuard: 防止重入攻击（虽然 createToken 不太可能被重入，但最佳实践是加上）
import "@openzeppelin/contracts/access/Ownable.sol";
import "@openzeppelin/contracts/utils/ReentrancyGuard.sol";

// ===== 导入之前写的代币合约 =====
// 注意路径：根据你的实际项目结构调整
import "./DeFiToken.sol";       // Day5 写的 ERC20（带 mint/burn）
import "./MyNFT.sol";           // Day9 写的 ERC721（带 metadata/enumerable）
import "./DeFiMultiToken.sol";  // Day10 写的 ERC1155（带 supply/burnable）

// ============================================
// TokenFactory — 统一代币工厂
// ============================================

contract TokenFactory is Ownable, ReentrancyGuard {

    // ===== 枚举：代币类型 =====
    // 为什么用 enum？
    //   1. 可读性好：Type.ERC20 比 0 清晰
    //   2. 类型安全：不能传非法值
    //   3. 可扩展：以后加 Type.ERC4626 直接往后加
    enum Type {
        ERC20,      // 0
        ERC721,     // 1
        ERC1155     // 2
    }

    // ===== Struct：记录创建的代币元信息 =====
    // 为什么用 struct 而不是分开的 mapping？
    //   把相关字段打包在一起，方便返回和检索
    struct TokenInfo {
        Type tokenType;         // 枚举占用 1 byte -> 打包 ！
        address tokenAddress;   // 20 bytes -> 打包 ！
        address creator;        // 20 bytes -> 不跟上面打包 (总和 > 32)
        string name;            // 动态类型 -> 存在别处
        string symbol;          // 动态类型 -> 存在别处
        uint256 createdAt;      // 32 bytes -> 单独 slot
    }
    // 存储布局：
    //   slot 0: [enum(1B) | address(20B)] ← 打包
    //   slot 1: creator(20B) → 浪费 12B（但没法优化，address 必须占独立 slot）
    //   slot 2: name 的长度（实际内容在 keccak256(slot2)）
    //   slot 3: symbol 的长度
    //   slot 4: createdAt(32B)
    //
    // 优化建议：如果把 createdAt 改成 uint48（够用到 8921556 年）
    //   就可以跟 creator 打包：slot 1: [creator(160b) | createdAt(48b)]

    // ===== 存储 =====
    // 代币列表：记录所有创建过的代币
    TokenInfo[] private _allTokens;

    // 快速查找：根据地址找到索引
    mapping(address => uint256) private _tokenIndex;    // tokenAddress => index+1 (0 表示不存在)

    // 用户维度
    mapping(address => uint256[]) private _userTokenIndices;

    // 按类型过滤
    mapping(Type => uint256[]) private _typeTokenIndices;

    // ===== 费用系统（可选：工厂的盈利模式）=====
    // 创建代币收 0.01 ETH 费用
    uint256 public createFee = 0.01 ether;

    // ===== 事件 =====
    // 每个事件为什么这么设计：
    //   indexed 参数：供链下过滤搜索
    //   非 indexed 参数：供链下读取详细信息
    event TokenCreated(
        address indexed creator,
        address indexed tokenAddress,
        Type indexed tokenType,         // indexed enum -> 可以按类型过滤事件
        string name,
        string symbol,
        uint256 timestamp
    );

    event CreationFeeUpdated(uint256 oldFee, uint256 newFee);
    event FeesWithdrawn(address indexed to, uint256 amount);

    // ===== Error =====
    // 自定义 error 比 revert string 省 gas
    error TokenAlreadyExists(address tokenAddress);
    error TokenNotFound(address tokenAddress);
    error InsufficientCreationFee(uint256 required, uint256 paid);
    error ZeroAddress();
    error EmptyName();
    error ZeroSupply();
    error NoFeesToWithdraw();
    error WithdrawFailed();

    // ===== 构造函数 =====
    constructor() Ownable(msg.sender) {
        // Ownable(msg.sender): OZ v5 的 Ownable 需要显式传 owner
        // msg.sender 是部署者，作为工厂的管理员
    }

    // ===== 核心函数：创建代币 =====

    // 创建 ERC20
    // @param _initialSupply 初始供应量（人类可读，如 1000 表示 1000 个代币）
    function createERC20(string calldata _name, string calldata _symbol, uint256 _initialSupply) external payable nonReentrant returns (address tokenAddr) {
        _validateInput(_name);
        if (_initialSupply == 0) revert ZeroSupply();
        _validateFee();

        // DeFiToken 构造函数签名: (name_, symbol_, initialSupply_)
        // 构造函数内部 _mint(msg.sender, initialSupply_)，msg.sender 是工厂本身
        // 所以部署后需要把代币转给实际创建者
        uint256 scaledSupply = _initialSupply * 10 ** 18;
        DeFiToken token = new DeFiToken(_name, _symbol, scaledSupply);
        tokenAddr = address(token);
        token.transfer(msg.sender, scaledSupply);

        _recordToken(Type.ERC20, tokenAddr, msg.sender, _name, _symbol);

        return tokenAddr;
    }

    // 创建 ERC721
    // @param _baseURI 元数据基础 URI（如 "ipfs://QmXxx...abc/"）
    function createERC721(string calldata _name, string calldata _symbol, string calldata _baseURI) external payable nonReentrant returns (address tokenAddr) {
        _validateInput(_name);
        _validateFee();

        // MyNFT 构造函数签名: (name_, symbol_, baseURI_)
        MyNFT nft = new MyNFT(_name, _symbol, _baseURI);
        tokenAddr = address(nft);

        _recordToken(Type.ERC721, tokenAddr, msg.sender, _name, _symbol);

        return tokenAddr;
    }

    function createERC1155(string calldata _name, string calldata _uri) external payable nonReentrant returns (address tokenAddr) {
        _validateInput(_name);
        _validateFee();

        // DeFiMultiToken 构造函数签名: (_name, baseURI_)
        DeFiMultiToken multiToken = new DeFiMultiToken(_name, _uri);
        tokenAddr = address(multiToken);

        // 注意: ERC1155 没有 symbol，这里 _uri 被存入 TokenInfo.symbol 字段（语义上复用）
        _recordToken(Type.ERC1155, tokenAddr, msg.sender, _name, _uri);

        return tokenAddr;
    }

    // ===== 内部辅助函数 =====

    // _validateInput: 验证输入参数
    function _validateInput(string calldata _name) internal pure {
        // bytes(_name).length: string 转成 bytes 后取长度
        // 不能用 _name.length（这返回的是 UTF-8 的字符数，有风险）
        if(bytes(_name).length == 0) revert EmptyName();
    }

    // _validateFee: 验证创建费
    function _validateFee() internal view {
        if(msg.value < createFee) revert InsufficientCreationFee({required: createFee, paid: msg.value});
    }

    // _recordToken: 记录新创建的代币
    function _recordToken(Type _type, address _tokenAddr, address _creator, string memory _name, string memory _symbol) internal {
        // 防止同一个地址被记录两次
        if (_tokenIndex[_tokenAddr] != 0) revert TokenAlreadyExists(_tokenAddr);

        // 创建 TokenInfo
        TokenInfo memory info = TokenInfo({
            tokenType: _type,
            tokenAddress: _tokenAddr,
            creator: _creator,
            name: _name,
            symbol: _symbol,
            createdAt: block.timestamp
        });

        // 加入数组
        _allTokens.push(info);

        // 索引 = 数组长度 (注意：push 后 length 已经 +1)
        uint256 index = _allTokens.length; // 1-based index
        // 为什么用 1-based index？
        //   因为 0 用于表示"不存在"
        //   _tokenIndex[addr] = 0 → 这个地址不是工厂创建的
        _tokenIndex[_tokenAddr] = index;

        // 用户维度
        _userTokenIndices[_creator].push(index);

        // 类型维度
        _typeTokenIndices[_type].push(index);

        // emit 事件
        emit TokenCreated(_creator, _tokenAddr, _type, _name, _symbol, block.timestamp);
    }

    // ===== 查询函数 =====

    // 根据地址获取代币信息
    // 返回值为什么用 TokenInfo memory？
    //   struct 在 memory 中临时构建，返回给调用者
    function getTokenInfo(address _tokenAddr) external view returns (TokenInfo memory) {
        uint256 index = _tokenIndex[_tokenAddr];
        if (index == 0) revert TokenNotFound(_tokenAddr);

        // index 是 1-based, 数组是 0-based, 所以 -1
        return _allTokens[index - 1];
    }

    // 获取总创建的代币数
    function getTotalTokens() external view returns (uint256) {
        return _allTokens.length;
    }

    // 获取所有代币（分页查询，防止一次返回太多数据）
    function getAllTokens(uint256 _offset, uint256 _limit) external view returns (TokenInfo[] memory result) {
        uint256 total = _allTokens.length;
        if (_offset >= total) return new TokenInfo[](0);

        uint256 end = _offset + _limit;
        if (end > total) end = total;

        uint256 size = end - _offset;
        result = new TokenInfo[](size);

        for (uint256 i = 0; i < size; i++) {
            result[i] = _allTokens[_offset + i];
        }
        // ⚠️ 注意：如果 _allTokens 很长，这个循环 gas 很高
        //   分页就是用来解决这个问题的（_limit 控制每页数量）
    }

    // 获取某用户创建的所有代币
    function getUserTokens(address _user) external view returns (TokenInfo[] memory result) {
        uint256[] storage indices = _userTokenIndices[_user];
        result = new TokenInfo[](indices.length);

        for (uint256 i = 0; i < indices.length; i++) {
            // indices[i] 是 1-based, 存的是 _allTokens 的索引
            result[i] = _allTokens[indices[i] - 1];
        }
    }

    // 获取某类型的代币
    function getTokensByType(Type _type, uint256 _offset, uint256 _limit) external view returns (TokenInfo[] memory result) {
        uint256[] storage indices = _typeTokenIndices[_type];
        if (_offset >= indices.length) return new TokenInfo[](0);

        uint256 end = _offset + _limit;
        if (end > indices.length) end = indices.length;

        uint256 size = end - _offset;
        result = new TokenInfo[](size);

        for (uint256 i = 0; i < size; i++) {
            result[i] = _allTokens[indices[_offset + i] - 1];
        }
    }

    // ===== 管理员函数 =====

    // 设置创建费
    function setCreationFee(uint256 _newFee) external onlyOwner {
        uint256 oldFee = createFee;
        createFee = _newFee;
        emit CreationFeeUpdated(oldFee, _newFee);
    }

    // 提取赚取的费用
    function withdrawFees() external onlyOwner nonReentrant {
        uint256 balance = address(this).balance;
        if (balance == 0) revert NoFeesToWithdraw();

        // CEI: 先更新状态（本函数无状态变更），再转账
        (bool ok, ) = owner().call{value: balance}("");
        if (!ok) revert WithdrawFailed();

        emit FeesWithdrawn(owner(), balance);
    }
}