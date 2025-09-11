const { expect } = require("chai")
const { ethers } = require("hardhat")

describe("RomaToInt 合约 - 罗马数字转整数测试", function () {
    let RomaToInt;
    let romaToIntInstance;

    beforeEach(async function () {
        RomaToInt = await ethers.getContractFactory("RomaToInt");
        romaToIntInstance = await RomaToInt.deploy();
    });

    // 测试用例 1: 输入 "III" -> 预期输出 3
    it("应该正确转换 'III' -> 3", async function () {
        const inputRoman = 'III';
        const expectInt = 3;

        // 调用后台合约转换函数
        const acutalInt = await romaToIntInstance.romanToInt(inputRoman);
        expect(acutalInt).to.equal(expectInt);
    });

    // 测试用例 1: 输入 "LVIII" -> 预期输出 58
    it("应该正确转换 'LVIII' -> 58", async function () {
        const inputRoman = 'LVIII';
        const expectInt = 58;

        // 调用后台合约转换函数
        const acutalInt = await romaToIntInstance.romanToInt(inputRoman);
        expect(acutalInt).to.equal(expectInt);
    });


    // 测试用例 1: 输入 "MCMXCIV" -> 预期输出 1994 (含特殊减算场景)
    it("应该正确转换 'MCMXCIV' -> 1994", async function () {
        const inputRoman = 'MCMXCIV';
        const expectInt = 1994;

        // 调用后台合约转换函数
        const acutalInt = await romaToIntInstance.romanToInt(inputRoman);
        expect(acutalInt).to.equal(expectInt);
    });
})