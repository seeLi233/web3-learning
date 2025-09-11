// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract ReverseString {
    // 反转字符串函数
    function reverse(string memory _str) public pure returns (string memory) {
        // 将字符串转换为字节数组
        bytes memory strBytes = bytes(_str);
        // 计算字符串长度
        uint256 length = strBytes.length;
        // 双指针交换实现反转
        for (uint256 i = 0; i < length / 2; i++) {
            // 交换对称位置的字符
            bytes1 temp = strBytes[i];
            strBytes[i] = strBytes[length - 1 - i];
            strBytes[length - 1 - i] = temp;
        }
        // 将字节数组转换回字符串
        return string(strBytes);
    }
}