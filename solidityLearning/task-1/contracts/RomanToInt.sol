// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract RomaToInt {
    // 映射：罗马字符 (bytes1 类型) -> 对应数值
    mapping(bytes1 => uint256) private _romanValues;

    // 构造函数：初始化罗马字符与数值的映射关系
    constructor() {
        _romanValues[bytes1('I')] = 1;
        _romanValues[bytes1('V')] = 5;
        _romanValues[bytes1('X')] = 10;
        _romanValues[bytes1('L')] = 50;
        _romanValues[bytes1('C')] = 100;
        _romanValues[bytes1('D')] = 500;
        _romanValues[bytes1('M')] = 1000;
    }

    /**
    * @dev 罗马数字转整数的核心函数
    * @param roman 输入的合法罗马数字字符串 (仅支持题目指定的字符： I/V/X/L/C/D/M)
    * @return 转换后的整数
    */
    function romanToInt(string memory roman) public view returns (uint256) {
        uint256 length = bytes(roman).length;
        uint256 result = 0;

        // 遍历罗马字符串（到倒数第二个字符，需与下一个字符比较）
        for(uint256 i = 0; i < length - 1; i++) {
            bytes1 currentChar = bytes(roman)[i];
            bytes1 nextChar = bytes(roman)[i + 1];

            uint256 currentVal = _romanValues[currentChar];
            uint256 nextVal = _romanValues[nextChar];

            // 规则：当前值 < 下一个值 -> 减当前值 (如 IV: 1 < 5 -> 减 1)
            // 否则 -> 加当前值 (如 VI: 5 > 1 -> 加 1)
            if(currentVal < nextVal) {
                result -= currentVal;
            } else {
                result += currentVal;
            }
        }

        // 加上最后一个字符的值(循环未处理最后一个)
        result += _romanValues[bytes(roman)[length - 1]];
        return result;
    }
}