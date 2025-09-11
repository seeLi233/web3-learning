const { expect } = require("chai")
const { ethers } = require("hardhat")

describe("IntegerToRoman 合约测试", function() {
    let IntegerToRoman;
    let integerToRomanInstance;

    beforeEach(async function () {
        IntegerToRoman = await ethers.getContractFactory("IntegerToRoman");
        integerToRomanInstance = await IntegerToRoman.deploy()
    });

    it("应该将 3749 转换为 'MMMDCCXLIX'", async function () {
        const result = await integerToRomanInstance.convertToRoman(3749);
        expect(result).to.equal("MMMDCCXLIX");
    })

    it("应该将 58 转换为 'LVIII'", async function () {
        const result = await integerToRomanInstance.convertToRoman(58);
        expect(result).to.equal("LVIII");
    })

    it("应该将 1994 转换为 'MCMXCIV'", async function () {
        const result = await integerToRomanInstance.convertToRoman(1994);
        expect(result).to.equal("MCMXCIV");
    })
})