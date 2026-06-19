// SPDX-License-Identifier: SEE LICENSE IN LICENSE
pragma solidity ^0.8.0;

// =============================================
// 13-MyToken.sol — 手写最简 ERC20（不用任何库，理解底层）
// =============================================

interface IERC20 {
    function totalSupply() external view returns (uint256);
    function balanceOf(address account) external view returns (uint256);
    function transfer(address to, uint256 amount) external returns (bool);
    function allowance(address owner, address spender) external view returns (uint256);
    function approve(address spender, uint256 amount) external returns (bool);
    function transferFrom(address from, address to, uint256 amount) external returns (bool);

    event Transfer(address indexed from, address indexed to, uint256 value);
    event Approval(address indexed owner, address indexed spender, uint256 value);
}

contract MyToken is IERC20 {
    // ===== 状态变量 =====
    string public name;
    string public symbol;
    uint8 public immutable decimals;  // immutable 比 constant 灵活，可在构造函数中赋值
    uint256 private _totalSupply;

    // mapping(address => uint256) 的存储位置: keccak256(key, slot)
    mapping(address => uint256) private _balances;
    mapping(address => mapping(address => uint256)) private _allowances;
    //                            ↑ spender  ↑ owner
    // 嵌套 mapping 的存储位置: keccak256(spender, keccak256(owner, slot))

    address public immutable owner;

    // ===== 构造函数 =====
    constructor(string memory _name, string memory _symbol, uint8 _decimals, uint256 initialSupply) {
        name = _name;
        symbol = _symbol;
        decimals = _decimals;
        owner = msg.sender;

        // 铸造初始供应量给部署者
        // 注意：decimals 是展示用的小数位，实际存储的是最小单位
        // 例如 1000 token with 18 decimals = 1000 * 10^18
        _totalSupply = initialSupply * (10 ** _decimals);
        _balances[msg.sender] = _totalSupply;
        emit Transfer(address(0), msg.sender, _totalSupply);
    }

    // ===== ERC20 核心函数 =====

    function totalSupply() external view override returns (uint256) {
        return _totalSupply;
    }

    function balanceOf(address account) external view override returns (uint256) {
        return _balances[account];
    }

    function transfer(address to, uint256 amount) external override returns (bool) {
        _transfer(msg.sender, to, amount);
        return true;
    }

    function allowance(address _owner, address spender) external view override returns (uint256) {
        return _allowances[_owner][spender];
    }

    function approve(address spender, uint256 amount) external override returns (bool) {
        _approve(msg.sender, spender, amount);
        return true;
    }

    function transferFrom(address from, address to, uint256 amount) external override returns (bool) {
        // 检查授权额度
        uint256 currentAllowance = _allowances[from][msg.sender];
        require(currentAllowance >= amount, "ERC20: insufficient allowance");

        // 扣减授权额度（如果授权了最大值 type(uint256).max，则不扣减）
        if (currentAllowance != type(uint256).max) {
            _approve(from, msg.sender, currentAllowance - amount);
        }

        _transfer(from, to, amount);
        return true;
    }

    // ===== 内部函数 =====

    function _transfer(address from, address to, uint256 amount) internal {
        require(from != address(0), "ERC20: transfer from zero address");
        require(to != address(0), "ERC20: transfer to zero address");
        require(_balances[from] >= amount, "ERC20: insufficient balance");

        _balances[from] -= amount;
        _balances[to] += amount;

        emit Transfer(from, to, amount);
    }

    function _approve(address _owner, address spender, uint256 amount) internal {
        require(_owner != address(0), "ERC20: approve from zero address");
        require(spender != address(0), "ERC20: approve to zero address");

        _allowances[_owner][spender] = amount;
        emit Approval(_owner, spender, amount);
    }

    // ===== 扩展功能 =====

    // 铸造：只有 owner 可以调用
    modifier onlyOwner() {
        require(msg.sender == owner, "MyToken: not owner");
        _;
    }

    function mint(address to, uint256 amount) external onlyOwner {
        require(to != address(0), "ERC20: mint to zero address");
        _totalSupply += amount;
        _balances[to] += amount;
        emit Transfer(address(0), to, amount);
    }

    // 燃烧：任何人可以销毁自己的代币
    function burn(uint256 amount) external {
        require(_balances[msg.sender] >= amount, "ERC20: burn amount exceeds balance");
        _balances[msg.sender] -= amount;
        _totalSupply -= amount;
        emit Transfer(msg.sender, address(0), amount);
    }
}