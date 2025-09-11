const { expect } = require("chai")
const { ethers } = require("hardhat")

describe("BinarySearch", function() {
    let BinarySearch;
    let binarySearch;

    beforeEach(async function () {
        BinarySearch = await ethers.getContractFactory("BinarySearch");
        binarySearch = await BinarySearch.deploy();
    });

    it("应该找到数组中的元素", async function () {
        const arr = [1, 3, 5, 7, 9];
        const target = 5;

        const result = await binarySearch.search(arr, target);
        expect(result).to.equal(2)
    });

    it("应该找到数组第一个元素", async function () {
    const arr = [2, 4, 6, 8, 10];
    const target = 2;
    const result = await binarySearch.search(arr, target);
    expect(result).to.equal(0);
  });

  it("应该找到数组最后一个元素", async function () {
    const arr = [1, 2, 3, 4, 5];
    const target = 5;
    const result = await binarySearch.search(arr, target);
    expect(result).to.equal(4);
  });

  it("应该返回-1当元素不存在时", async function () {
    const arr = [10, 20, 30, 40, 50];
    const target = 25;
    const result = await binarySearch.search(arr, target);
    expect(result).to.equal(-1);
  });

  it("应该处理空数组", async function () {
    const arr = [];
    const target = 5;
    const result = await binarySearch.search(arr, target);
    expect(result).to.equal(-1);
  });

  it("应该处理单元素数组（找到元素）", async function () {
    const arr = [7];
    const target = 7;
    const result = await binarySearch.search(arr, target);
    expect(result).to.equal(0);
  });

  it("应该处理单元素数组（未找到元素）", async function () {
    const arr = [7];
    const target = 3;
    const result = await binarySearch.search(arr, target);
    expect(result).to.equal(-1);
  });

  it("应该处理包含负数的数组", async function () {
    const arr = [-5, -3, 0, 2, 4];
    const target = -3;
    const result = await binarySearch.search(arr, target);
    expect(result).to.equal(1);
  });
})