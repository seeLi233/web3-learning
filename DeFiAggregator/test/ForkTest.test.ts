import { expect } from "chai";
import { network } from "hardhat";

// ==================== 连接 Fork 网络 ====================
// ⚠️ 关键区别：network.create() 传 "mainnetFork" → 连接主网 Fork
//    不传参数 → 连接 default 本地空网络
//    所以这个测试文件的"世界"里，有真实主网的全部状态
const { ethers } = await network.create("mainnetFork");

describe("🍴 Fork 测试 — 真实主网状态验证", function () {
    // ==================== 主网真实合约地址 ====================
    // 为什么写死地址而不是部署 mock？
    // → fork 测试的核心价值就是"和真实合约交互"，这些是 canonical 地址，永久不变
    const USDC = "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48"; // 6 位小数
    const WETH = "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2"; // 18 位小数
    const DAI  = "0x6B175474E89094C44Da98b954EedeAC495271d0F";  // 18 位小数

    // ==================== 最小 ERC20 ABI ====================
    // 为什么用内联 ABI 而不是 getContractFactory("ERC20")？
    // → USDC/WETH/DAI 不是本项目合约，本地没有它们的 artifacts
    //    内联 ABI 只声明我要调用的函数签名，足够读取状态
    // 为什么用 view 函数（decimals/totalSupply/balanceOf）？
    // → 这些是读操作，不消耗 gas、不需要签名者，fork 测试里最快最稳
    const erc20Abi = [
        "function decimals() view returns (uint8)",
        "function totalSupply() view returns (uint256)",
        "function balanceOf(address) view returns (uint256)",
    ];

    // ==================== A 组 — 精度验证 ====================
    describe("A. 精度验证 — 真实代币的小数位", function () {
        it("A1. USDC 是 6 位小数，WETH/DAI 是 18 位小数", async function () {
            // 为什么 new ethers.Contract 而不是 deployContract？
            // → deployContract 是"部署新合约"；这里合约已在主网存在，
            //    我们要做的是"连接到已有地址"，用 Contract 构造器包装即可
            const usdc = new ethers.Contract(USDC, erc20Abi, ethers.provider);
            const weth = new ethers.Contract(WETH, erc20Abi, ethers.provider);
            const dai  = new ethers.Contract(DAI,  erc20Abi, ethers.provider);

            // 断言真实精度：这是 fork 测试最经典的"真相校验"
            // 如果这里返回错误值，说明你的 RPC 没连上或 fork 配置有问题
            expect(await usdc.decimals()).to.equal(6);
            expect(await weth.decimals()).to.equal(18);
            expect(await dai.decimals()).to.equal(18);
        });

        it("A2. 精度差异如何影响金额表示（聚合器必考陷阱）", async function () {
            // 为什么演示这个？→ 你是聚合器开发者，聚合器要同时处理 USDC(6位)
            //    和 DAI(18位)，把 1 USDC 和 1 DAI 都当 18 位算，会差 10^12 倍
            // 作用：用 parseUnits 展示"同一个数量，不同精度下的数值完全不同"
            const oneUSDC = ethers.parseUnits("1", 6);    // 1 USDC = 1_000_000 (wei 单位)
            const oneDAI  = ethers.parseUnits("1", 18);   // 1 DAI  = 1_000_000_000_000_000_000

            // 为什么 USDC 的"1"是 10^6，DAI 的"1"是 10^18？
            // → EVM 里所有金额都是整数，小数位由 token 合约的 decimals 决定
            //    USDC 把 1 美元拆成 100 万份，DAI 拆成 10^18 份
            expect(oneUSDC).to.equal(1_000_000n);
            expect(oneDAI).to.equal(10n ** 18n);
        });
    });

    // ==================== B 组 — 真实总量与余额 ====================
    describe("B. 真实总量与余额 — 主网不是空的", function () {
        it("B1. USDC 和 WETH 的总发行量 > 0", async function () {
            const usdc = new ethers.Contract(USDC, erc20Abi, ethers.provider);
            const weth = new ethers.Contract(WETH, erc20Abi, ethers.provider);

            // 为什么断言 > 0 而不是具体数值？
            // → 总量会随铸造/销毁变化，断言">0"证明读到的是真实非空状态
            expect(await usdc.totalSupply()).to.be.gt(0n);
            expect(await weth.totalSupply()).to.be.gt(0n);
        });

        it("B2. WETH 合约自己持有大量 WETH（真实余额非零）", async function () {
            // 为什么读 WETH.balanceOf(WETH) 这个地址？
            // → 人们把 ETH 包成 WETH 时，ETH 存在 WETH 合约里，
            //    WETH 合约自己账上永远有巨额 WETH，是一个稳定的非零余额来源
            const weth = new ethers.Contract(WETH, erc20Abi, ethers.provider);

            const wethSelfBalance = await weth.balanceOf(WETH);
            expect(wethSelfBalance).to.be.gt(0n);

            // 打印出来让你直观感受"真实主网的钱"有多大
            console.log(
                "  🔍 WETH 合约自持 WETH =",
                ethers.formatEther(wethSelfBalance),
                "WETH"
            );
        });
    });
});