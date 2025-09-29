// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract BeggingContract {
    // 合约所有者地址
    address private immutable owner;

    // 记录每个地址的捐赠总金额
    mapping(address => uint256) public donations;

    // 捐赠事件，记录每次捐赠的地址和金额
    event DonationReceived(address indexed donor, uint256 amount);

    // 捐赠排行榜 - 记录捐赠金额最多的前 3 个地址
    address[3] public topDonors;
    uint256[3] public topDonations;

    // 时间限制 - 只有在特定时间段内才能捐赠
    uint256 public donationStart;
    uint256 public donationEnd;

    // 修饰符：仅所有者可调用
    modifier onlyOwner() {
        require(msg.sender == owner, "Only contract owner can call this function");
        _;
    }

    // 修饰符：检查是否在捐赠时间范围内
    modifier validDonationTime() {
        // 如果设置了时间限制，则检查是否在有效时间内
        if (donationStart > 0 && donationEnd > 0) {
            require(block.timestamp >= donationStart && block.timestamp <= donationEnd, "Donations are only allowed during the specified period");
        }
        _;
    }

    // 构造函数，设置合约部署者为所有者
    constructor () {
        owner = msg.sender;
    }

    function getOwner() public view returns (address) {
        return owner;
    }

    // 捐赠函数，允许用户向合约发送以太币
    function donate() public payable validDonationTime {
        require(msg.value > 0, "Donation amount must be greater than 0");

        // 更新捐赠记录
        donations[msg.sender] += msg.value;

        // 更新捐赠排行榜
        updateTopDonors(msg.sender, donations[msg.sender]);

        // 触发捐赠事件
        emit DonationReceived(msg.sender, msg.value);
    }

    // 更新捐赠排行榜
    function updateTopDonors(address donor, uint256 totalDonation) private {
        // 检查是否进入前三
        for(uint i = 0; i < 3; i++) {
            if(totalDonation > topDonations[i]) {
                // 移位
                for(uint j = 2; j > i; j--) {
                    topDonors[j] = topDonors[j - 1];
                    topDonations[j] = topDonations[j - 1];
                }
                // 插入新的捐赠者
                topDonors[i] = donor;
                topDonations[i] = totalDonation;
                break;
            }
        }
    }

    // 提取所有资金，仅所有者可调用
    function withdraw() public onlyOwner {
        uint256 balance = address(this).balance;
        require(balance > 0, "No funds available to withdraw");

        // 转账给所有者
        payable(owner).transfer(balance);
    }

    // 查询某个地址的捐赠金额
    function getDonation(address donor) public view returns (uint256) {
        return donations[donor];
    }

    // 获取合约当前余额
    function getContractBalance() public view returns (uint256) {
        return address(this).balance;
    }

    // 设置捐赠时间范围（可选功能）
    function setDonationPeriod(uint256 start, uint256 end) public onlyOwner {
        require(start < end, "Start time must be before end time");

        donationStart = start;
        donationEnd = end;
    }

    // 接收以太币的回调函数，确保直接转账也能被记录
    receive() external payable {
        donate();
    }
}