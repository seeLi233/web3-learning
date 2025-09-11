const { expect } = require("chai")
const { ethers } = require("hardhat")

describe("ReverseString 合约 - 字符串反转功能测试", function() {
    let ReverseString;
    let reverseStringInstance;

    beforeEach(async function () {
        // 获取合约工厂
        ReverseString = await ethers.getContractFactory("ReverseString");
        // 合约部署
        reverseStringInstance = await ReverseString.deploy();
    });

    it("应该正确反转普通ASCII字符串: 输入'abcde' -> 输出'edcba'", async function () {
        // 定义测试输入和预期输出
        const inputStr = "abcde";
        const expectOutput = "edcba";

        // 调用合约的 reverse 函数
        const acutalOutput = await reverseStringInstance.reverse(inputStr);
        // 断言：实际输出与预期一样
        expect(acutalOutput).to.equal(expectOutput);
    });

    it("应该正确处理空字符串: 输入'' -> 输出''", async function () {
        // 定义测试输入和预期输出
        const inputStr = "";
        const expectOutput = "";

        // 调用合约的 reverse 函数
        const acutalOutput = await reverseStringInstance.reverse(inputStr);
        // 断言：实际输出与预期一样
        expect(acutalOutput).to.equal(expectOutput);
    });

    it("应该正确反转数字+特殊符号字符串: 输入'123!@#' -> 输出'#@!321'", async function () {
        // 定义测试输入和预期输出
        const inputStr = "123!@#";
        const expectOutput = "#@!321";

        // 调用合约的 reverse 函数
        const acutalOutput = await reverseStringInstance.reverse(inputStr);
        // 断言：实际输出与预期一样
        expect(acutalOutput).to.equal(expectOutput);
    });
})