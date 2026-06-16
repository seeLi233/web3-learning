// SPDX-License-Identifier: SEE LICENSE IN LICENSE
pragma solidity ^0.8.0;

/**
 * @title Mappings
 * @notice 学习 Solidity Mapping
 */
contract Mappings {
    // 基础 Mapping
    // address => uint256
    mapping (address => uint256) public balances;

    // address => bool
    mapping (address => bool) public isWhitelisted;

    // uint256 => address
    mapping (uint256 => address) public tokenOwners;

    // 嵌套 Mapping
    // address => (address => uint256)
    mapping (address => mapping (address => uint256)) public allowance;

    // 带结构体的 Mapping
    struct Proposal {
        string descrition;
        uint256 voteCount;
        uint256 deadline;
        bool executed;
    }

    mapping (uint256 => Proposal) public proposals;
    uint256 public proposalCount;

    // 函数演示
    function deposite() external payable {
        balances[msg.sender] += msg.value;
    }

    function withdraw(uint256 amount) external {
        require(balances[msg.sender] >= amount, "Insufficient balance");
        balances[msg.sender] -= amount;
        payable(msg.sender).transfer(amount);
    }

    function addToWhitelist(address account) external {
        isWhitelisted[account] = true;
    }

    function createProposal(string calldata description, uint256 duration) external {
        proposalCount++;
        proposals[proposalCount] = Proposal({
            descrition: description,
            voteCount: 0,
            deadline: block.timestamp + duration,
            executed: false
        });
    }

    // 注意 mapping 不能遍历！
    // 如果需要遍历，需要额外维护一个数组
}