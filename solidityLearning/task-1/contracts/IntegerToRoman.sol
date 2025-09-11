// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract IntegerToRoman {
    // 定义所有可能得数值 (从大到小排序)
    uint256[] private values = [
        1000, 900, 500, 400,
        100, 90, 50, 40,
        10, 9, 5, 4, 1
    ];

    // 对应的罗马数字符号 (与 values 数组一一对应)
    string[] private symbols = [
        "M", "CM", "D", "CD",
        "C", "XC", "L", "XL",
        "X", "IX", "V", "IV",
        "I"
    ];

    /**
    * @dev 将整数转换为罗马数字
    * @param num 输入的整数，必须在 1 到 3999 之间
    * @return 对应的罗马数字字符串
    */
    function convertToRoman(uint256 num) public view returns (string memory) {
        // 验证输入范围，罗马数字通常用于表示 1到 3999 之间的数
        require(num >= 1 && num <= 3999, "Number must be between 1 and 3999");

        string memory result = "";

        // 贪心算法：从最大的数值开始匹配
        for(uint256 i = 0; i < values.length; i++) {
            // 当当前值小于等于剩余数字时，添加对应的罗马字符
            while(values[i] <= num) {
                // 从符号添加到结果中
                result = string(abi.encodePacked(result, symbols[i]));
                // 减去已匹配的数值
                num -= values[i];
            }

            // 数字减为 0 时退出循环
            if (num == 0) {
                break;
            }
        }

        return result;
    }
}