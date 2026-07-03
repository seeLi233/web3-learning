// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract Auction {

    // 自定义 error
    error AuctionNotActive();
    error BidTooLow(uint256 yourBid, uint256 currentHighest);
    error AuctionNotEnded();
    error AuctionAlreadyEnded();
    error NotOwner();
    error WithdrawFailed();
    error NothingToWithdraw();
    error AuctionNotEndedYet();
    error NoBidsPlaced();
    error TransferToOwnerFailed();
    error NotSeller();

    // 状态变量
    address public immutable owner;
    string public item;
    uint256 public minBid;
    uint256 public endTime;
    bool public ended;

    address public highestBidder;
    uint256 public highestBid;

    // 被反超的人， 钱暂存这里等退款
    mapping (address => uint256) public pendingReturns;

    // 记录所有参与出价的地址（用于查询）
    address[] public allBidders;
    mapping (address => bool) public isBidder; // 去重用

    // Events
    event NewBid(address indexed bidder, uint256 amount);
    event Withdrawal(address indexed bidder, uint256 amount);
    event AuctionEnded(address indexed winner, uint256 amount);

    // Modifier
    modifier onlyOwner() {
        if (msg.sender != owner) revert NotOwner();
        _;
    }

    modifier auctionActive() {
        if (block.timestamp >= endTime) revert AuctionNotActive();
        _;
    }

    modifier auctionOver() {
        if (block.timestamp <= endTime) revert AuctionNotEnded();
        if (ended) revert AuctionAlreadyEnded();
        _;
    }

    // constructor
    constructor(string memory _item, uint256 _minBid, uint256 _durationMinutes) {
        owner = msg.sender;
        item = _item;
        minBid = _minBid;
        endTime = block.timestamp + (_durationMinutes * 1 minutes);
    }

    /// @notice 出价
    /// @dev 自动退回被超出的前最高出价
    function bid() public payable auctionActive {
        // 计算本轮最低出价：第一轮用 minBid，之后用最高价 + 1
        uint256 minRequired = highestBid > 0 ? highestBid + 1 : minBid;
        if (msg.value < minRequired) {
            revert BidTooLow(msg.value, minRequired);
        }

        // 把前一个最高出价者的钱标记为待退款
        if (highestBidder != address(0)) {
            pendingReturns[highestBidder] += highestBid;
        }

        // 记录新出价者（可迭代 mapping 模式：去重）
        if (!isBidder[msg.sender]) {
            isBidder[msg.sender] = true;
            allBidders.push(msg.sender);
        }

        // 更新最高出价
        highestBidder = msg.sender;
        highestBid = msg.value;

        emit NewBid(msg.sender, msg.value);
    }

    // 被反超者取回退款
    function withdraw() public {
        uint256 amount = pendingReturns[msg.sender];
        if (amount == 0) revert NothingToWithdraw();

        // 先清零再转账（防重入攻击）
        pendingReturns[msg.sender] = 0;

        (bool success, ) =msg.sender.call{value: amount}("");
        if (!success) revert WithdrawFailed();

        emit Withdrawal(msg.sender, amount);
    }

    // 结束拍卖
    function endAuction() public auctionOver {
        ended = true;
        emit AuctionEnded(highestBidder, highestBid);
    }

    // Owner 提取资金
    function ownerWithdraw() public onlyOwner {
        if (!ended) revert AuctionNotEndedYet();
        if (highestBid == 0) revert NoBidsPlaced();

        uint256 amount = highestBid;
        highestBid = 0; // CEI: 先更新状态（防重入）

        (bool success, ) = owner.call{value:amount}("");
        if (!success) revert TransferToOwnerFailed();
    }

    // 视图函数
    function timeRemaining() public view returns (uint256) {
        if (block.timestamp >= endTime) return 0;
        return endTime - block.timestamp;
    }

    function bidderCount() public view returns (uint256) {
        return allBidders.length;
    }

    function getStatus() public view returns (string memory _item, uint256 _highestBid, address _highestBidder, uint256 _timeRemaining, bool _ended) {
        _item = item;
        _highestBid = highestBid;
        _highestBidder = highestBidder;
        _timeRemaining = timeRemaining();
        _ended = ended;
    }
}