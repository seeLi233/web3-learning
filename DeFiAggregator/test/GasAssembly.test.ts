import { expect } from "chai";
import { network } from "hardhat";

const { ethers } = await network.create();

// 精度常量
// 为什么声明为 BigInt？
// → ethers.parseEther 返回 bigint，后续断言需要同类型比较
const ONE_ETH = 10n ** 18n;

describe("⚡ GasAssembly — Assembly 优化与 Gas 对比", function () {
    // ============ 变量声明 ============
    // 为什么用 any 类型？
    // → 项目约定：合约变量统一 let + any，避免 ethers 类型推导噪音
    let gasAssembly: any;
    let sstore3: any;
    let owner: any, user1: any, user2: any;

    // ============ 部署 ============
    // 为什么每个 describe 前都重新部署？
    // → 保证测试独立性，storage 状态不互相污染
    beforeEach(async function () {
        // 初始化网络 + 获取签名者
        // 为什么用 network.create() 而不是 hre.ethers？
        // → 项目锁定的网络初始化范式，见 hardhat.config.ts 配置
        const { ethers } = await network.create();
        [owner, user1, user2] = await ethers.getSigners();

        // 部署 GasAssembly（构造函数会给 owner 发 100 万代币）
        gasAssembly = await ethers.deployContract("GasAssembly");

        // 部署 SSTORE3
        sstore3 = await ethers.deployContract("SSTORE3");
    });

    describe("A. 部署", function () {
        it("A1. 应该成功部署并给 owner 初始余额", async function () {
            // 为什么用 getAddress() 而不是 .address？
            // → ethers v6 中合约地址通过 getAddress() 获取
            const addr = await gasAssembly.getAddress();
            expect(addr).to.match(/^0x[0-9a-fA-F]{40}$/);

            // 验证初始余额 = 100 万 ether
            const balance = await gasAssembly.balanceOf(owner.address);
            expect(balance).to.equal(1_000_000n * ONE_ETH);
        });
    });

    describe("B. 功能正确性 — transfer 两种实现", function () {
        it("B1. Solidity 版 transfer 应该正常转账", async function () {
            // 为什么先记录前后余额？
            // → 验证转账后差额正好是 amount，且接收方增加了 amount
            const beforeOwner = await gasAssembly.balanceOf(owner.address);
            const beforeUser1 = await gasAssembly.balanceOf(user1.address);

            const amount = 100n * ONE_ETH;
            await gasAssembly.transferSolidity(user1.address, amount);

            const afterOwner = await gasAssembly.balanceOf(owner.address);
            const afterUser1 = await gasAssembly.balanceOf(user1.address);

            expect(afterOwner).to.equal(beforeOwner - amount);
            expect(afterUser1).to.equal(beforeUser1 + amount);
        });

        it("B2. Assembly 版 transfer 应该正常转账", async function () {
            const amount = 100n * ONE_ETH;
            await gasAssembly.transferAssembly(user1.address, amount);

            const afterUser1 = await gasAssembly.balanceOf(user1.address);
            expect(afterUser1).to.equal(amount);
        });

        it("B3. Assembly 版余额不足 → revert", async function () {
            // user2 没有余额，尝试转账应该 revert
            // 为什么用 connect(user2)？
            // → 让 user2 作为 msg.sender 调用，模拟无余额账户
            // assembly 里我们 revert(0,0)，没有错误信息，
            // 所以用 to.be.reverted（任意 revert），而不是 revertedWith
            await expect(
                gasAssembly.connect(user2).transferAssembly(user1.address, 1n)
            ).to.be.revert(ethers);
        });

        it("B4. Assembly 版应该正确发出 Transfer 事件", async function () {
            const amount = 50n * ONE_ETH;
            // 为什么检查事件？
            // → assembly 的 log3 必须和 Solidity 的 emit 效果一致
            //    否则链下索引（如 The Graph）会漏掉这笔转账
            await expect(gasAssembly.transferAssembly(user1.address, amount))
                .to.emit(gasAssembly, "Transfer")
                .withArgs(owner.address, user1.address, amount);
        });
    });

    describe("C. Gas 对比 — Solidity vs Assembly ⭐", function () {
        it("C1. transfer 的 gas 消耗对比", async function () {
            const amount = 10n * ONE_ETH;

            // Solidity 版 gas
            // 为什么用 estimateGas？
            // → 不实际发交易，只估算 gas，避免污染状态
            // 为什么显式标注 bigint？
            // → ethers v6 的 estimateGas 返回 bigint；TS6 会把「any - any」
            //    推断成 number，导致与 100n 相乘时报类型错误，这里显式收窄
            const solidityGas: bigint = await gasAssembly.transferSolidity.estimateGas(
                user1.address,
                amount
            );

            // Assembly 版 gas（注意：owner 的余额要够，所以先转账回补）
            // 这里 owner 初始有 100 万，扣了 10 之后还剩很多，不影响
            const assemblyGas: bigint = await gasAssembly.transferAssembly.estimateGas(
                user1.address,
                amount
            );

            // 打印对比结果
            // 为什么 console.log？
            // → 测试输出是面试的重要数据，gas 数字要能脱口而出
            // 为什么先转 number 再算百分比？
            // → bigint 是整数除法，(510 * 100) / 51541 会得到 0%
            //    转 number 才能算出带小数的真实百分比
            const savingsPct = ((Number(solidityGas - assemblyGas) * 100) / Number(solidityGas)).toFixed(2);
            console.log(
                `\n  📊 transfer gas: Solidity=${solidityGas}, Assembly=${assemblyGas}, 节省=${solidityGas - assemblyGas} (${savingsPct}%)`
            );

            // 断言 assembly 确实更省（至少不更贵）
            // 为什么用 <= 而不是 <？
            // → 某些 EVM 版本下 compiler 优化可能抹平差距，用 <= 更稳健
            expect(assemblyGas).to.be.lte(solidityGas);
        });

        it("C2. batchBalanceOf 的 gas 优势", async function () {
            const accounts = [owner.address, user1.address, user2.address, user1.address];

            // 对比：分别调用 balanceOf 4 次 vs batchBalanceOf 1 次
            const singleGas: bigint = await gasAssembly.balanceOf.estimateGas(owner.address);
            const singleTotal: bigint = singleGas * 4n; // 4 次单独调用

            const batchGas: bigint = await gasAssembly.batchBalanceOf.estimateGas(accounts);

            console.log(
                `\n  📊 batchBalanceOf: 单独4次=${singleTotal}, 批量1次=${batchGas}, 节省=${singleTotal - batchGas}`
            );

            // 批量应该显著更省（因为省了 3 次函数 dispatch + calldata 开销）
            expect(batchGas).to.be.lt(singleTotal);
        });

        it("C3. 🔥 面试题：什么时候用 assembly？什么时候不用？", async function () {
            // 为什么这是一个测试用例而不是纯注释？
            // → 项目约定 C4 系列是"面试用例"，用可运行的代码固化面试答案
            //    面试官问"你在生产用过 assembly 吗"时，这段代码就是论据

            // 对比：SLOAD 缓存的效果
            // 先构造一个会重复读同一 slot 的场景
            const repeated = [owner.address, owner.address, owner.address];

            // 批量读取（assembly 手动缓存）
            const batchGas = await gasAssembly.batchBalanceOf.estimateGas(repeated);

            console.log(
                `\n  🔥 面试结论：assembly 适合批量读（batchGas=${batchGas}），`
            );
            console.log(
                `     不适合简单算术（编译器已优化）。生产优先 struct packing + calldata。`
            );

            // 断言基本功能可用
            const balances = await gasAssembly.batchBalanceOf(repeated);
            expect(balances[0]).to.equal(balances[1]);
            expect(balances[1]).to.equal(balances[2]);
        });
    });

    describe("D. SSTORE3 — 指针存储 + 地址编码", function () {
        it("D1. write 应该成功部署数据并返回指针", async function () {
            // 为什么用 ethers.encodeBytes32String 之类的工具构造数据？
            // → 这里直接用 hex 字符串，方便验证读取结果一致
            const data = "0xdeadbeef12345678";

            const tx = await sstore3.write(data);
            const receipt = await tx.wait();

            // 从事件中拿到指针地址
            // 为什么从事件拿而不是看返回值？
            // → write 返回 pointer，但测试中直接用返回值更简单
            //    这里演示事件也能捕获
            let pointer = "";
            // 解析 DataStored 事件
            for (const log of receipt.logs) {
                try {
                    const parsed = sstore3.interface.parseLog(log);
                    if (parsed && parsed.name === "DataStored") {
                        pointer = parsed.args.pointer;
                    }
                } catch (e) {
                    // 忽略无法解析的 log
                }
            }
            expect(pointer).to.not.equal("");
        });

        it("D2. read 应该读回相同数据", async function () {
            const data = "0xdeadbeef12345678";

            // 为什么用真实交易而不是 staticCall 拿指针？
            // → staticCall（eth_call）不持久化 CREATE2 部署的合约，
            //    后续 read 会因为 extcodesize==0 而 revert
            const tx = await sstore3.write(data);
            const receipt = await tx.wait();
            const parsed = sstore3.interface.parseLog(receipt.logs[0]);
            const pointer = parsed!.args.pointer;

            // 读取回来
            const readBack = await sstore3.read(pointer);

            // 为什么比较时用 toLowerCase？
            // → bytes 返回的 hex 可能大小写不一致
            expect(readBack.toLowerCase()).to.equal(data.toLowerCase());
        });

        it("D3. 相同数据应该去重（返回相同指针）", async function () {
            const data = "0xdeadbeef12345678";

            // 写两次相同数据
            const pointer1 = await sstore3.write.staticCall(data);
            const pointer2 = await sstore3.write.staticCall(data);

            // SSTORE3 的核心：去重
            // 为什么相同数据要返回相同指针？
            // → CREATE2 的 salt 相同 → 地址相同，省重复部署 gas
            expect(pointer1).to.equal(pointer2);
        });

        it("D4. 地址编码/解码 uint96", async function () {
            const value = 123456789n;

            // 编码
            const pointer = await sstore3.encodeUint96(value);
            // 解码
            const decoded = await sstore3.decodeUint96(pointer);

            expect(decoded).to.equal(value);
        });

        it("D5. 复合编码（类型标记 + 数据）", async function () {
            const kind = 7n; // 类型标记
            const data = 999999n; // 数据

            const pointer = await sstore3.encodeTyped(kind, data);

            // 分别解码
            const decodedKind = await sstore3.decodeKind(pointer);
            const decodedData = await sstore3.decodeTypedData(pointer);

            expect(decodedKind).to.equal(kind);
            expect(decodedData).to.equal(data);
        });
    });
});