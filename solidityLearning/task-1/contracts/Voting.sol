// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

// 创建一个名为Voting的合约，包含以下功能：
//     一个mapping来存储候选人的得票数
//     一个vote函数，允许用户投票给某个候选人
//     一个getVotes函数，返回某个候选人的得票数
//     一个resetVotes函数，重置所有候选人的得票数

contract Voting {
    // 存储候选人的票数，键为候选人名称，值为得票数
    mapping(string => uint256) private _votes;

    // 存储所有候选人的名称，用于重置功能
    string[] private _candidates;

    // 记录候选人是否已存在，避免重复添加
    mapping(string => bool) private _candidateExists;

    //  合约所有者地址
    address private _owner;

    // 对候选人进行投票
    function vote(string calldata canidate) external {
        // 
        if (!_candidateExists[canidate]) {
            _candidates.push(canidate);
            _candidateExists[canidate] = true;
        }
        // 为候选人增加一票
        _votes[canidate]++;
    }

    // 
    function getVotes(string calldata candidate) external view returns (uint256) {
        return _votes[candidate];
    }

    //
    function resetVotes() external {
        // 
        // require(msg.sender == _owner, "Only owner can reset votes");

        //
        for (uint256 i = 0; i < _candidates.length; i++) {
            _votes[_candidates[i]] = 0;
        }
    }

    // 
    function getCandidates() external view returns (string[] memory) {
        return _candidates;
    }

    // 
    function isOwner() external view returns (bool) {
        return msg.sender == _owner;
    }
}