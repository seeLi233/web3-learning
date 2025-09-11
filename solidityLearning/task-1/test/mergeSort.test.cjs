const { expect } = require("chai")
const { ethers } = require("hardhat")

describe("MergeSortedArrays", function() {
    let MergeSortedArrays;
    let mergeSortedArrays;

    beforeEach(async function () {
        MergeSortedArrays = await ethers.getContractFactory("MergeSortedArrays");
        mergeSortedArrays = await MergeSortedArrays.deploy();
    });

    function convertBigIntArrayToNumbers(bigIntArray) {
        return bigIntArray.map(num => Number(num));
    }

    it("应该正常合并两个长度相同的有序数组", async function () {
        const arr1 = [1, 3, 5, 7];
        const arr2 = [2, 4, 6, 8];
        const expectedResult = [1, 2, 3, 4, 5, 6, 7, 8];

        const result = await mergeSortedArrays.merge(arr1, arr2);
        expect(result).to.deep.equal(expectedResult);
    });

    it("应该正确处理第一个数组为空的情况", async function () {
        const arr1 = [];
        const arr2 = [2, 4, 6, 8];
        const expectedResult = [2, 4, 6, 8];

        const result = await mergeSortedArrays.merge(arr1, arr2);
        expect(result).to.deep.equal(expectedResult);
    });

    it("应该正确处理第二个数组为空的情况", async function () {
        const arr1 = [1, 3, 5, 7];
        const arr2 = [];
        const expectedResult = [1, 3, 5, 7];

        const result = await mergeSortedArrays.merge(arr1, arr2);
        expect(result).to.deep.equal(expectedResult);
    });

    it("应该正确处理长度不同的数组", async function () {
        const arr1 = [1, 3, 5];
        const arr2 = [2, 4, 6, 8, 10];
        const expectedResult = [1, 2, 3, 4, 5, 6, 8, 10];

        const result = await mergeSortedArrays.merge(arr1, arr2);
        expect(result).to.deep.equal(expectedResult);
    });

    it("应该正确包含重复元素的数组", async function () {
        const arr1 = [2, 2, 3];
        const arr2 = [1, 2, 4];
        const expectedResult = [1, 2, 2, 2, 3, 4];

        const result = await mergeSortedArrays.merge(arr1, arr2);
        expect(result).to.deep.equal(expectedResult);
    });

    it("应该正确包含重复零的数组", async function () {
        const arr1 = [0, 2, 4];
        const arr2 = [1, 3, 5];
        const expectedResult = [0, 1, 2, 3, 4, 5];

        const result = await mergeSortedArrays.merge(arr1, arr2);
        expect(result).to.deep.equal(expectedResult);
    });
})