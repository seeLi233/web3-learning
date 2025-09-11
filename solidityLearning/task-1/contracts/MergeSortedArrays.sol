// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract MergeSortedArrays {
    // 合并两个有序数组， 返回一个有序数组
    function merge(uint256[] memory arr1, uint256[] memory arr2) public pure returns (uint256[] memory) {
        // 初始化指针
        uint256 i = 0;
        uint256 j = 0;
        uint256 k = 0;

        // 创建结果数组，长度为两个输入数组长度之和
        uint256[] memory result = new uint256[](arr1.length + arr2.length);

        // 比较两个数组的元素，将较小的元素放入结果数组
        while(i < arr1.length && j < arr2.length) {
            if(arr1[i] <= arr2[j]) {
                result[k] = arr1[i];
                i++;
            } else {
                result[k] = arr2[j];
                j++;
            }
            k++;
        }

        // 将arr1中剩余的元素添加到结果数组
        while(i < arr1.length) {
            result[k] = arr1[i];
            i++;
            k++;
        }

        // 将arr2中剩余的元素添加到结果数组
        while(j < arr2.length) {
            result[k] = arr2[j];
            j++;
            k++;
        }

        return result;
    }
}