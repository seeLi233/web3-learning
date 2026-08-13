import { expect } from "chai";
import { network } from "hardhat";

// ==================== 连接默认本地网络 ====================
// 为什么这里 network.create() 不传网络名？
// → 高级测试技巧（快照/时间/冒充）不依赖主网状态，本地网络更快更稳
// 注意这里解构出 networkHelpers —— 它是 Hardhat 3 内置的测试工具集
const { ethers, networkHelpers } = await network.create();

describe("🧪 高级测试技巧 — 快照 / 时间 / 冒充", function () {
    let owner: any, user1: any, user2: any;

    before(async function () {
        [owner, user1, user2] = await ethers.getSigners();
    });

    // ==================== A 组 — 快照/回滚 ====================
    describe("A. Snapshot/Revert — 状态存档与读档", function () {
        it("A1. 快照后改状态，回滚后恢复原状", async function () {
            // 部署一个带可变状态的合约（复用 GasComparison）
            const gasComparison = await ethers.deployContract("GasComparison");

            // 为什么先 snapshot？
            // → evm_snapshot 给当前整个 EVM 状态拍快照，返回一个 id
            //    作用：记下一个"存档点"，后面无论怎么改都能读档回来
            const snapshotId = await ethers.provider.send("evm_snapshot", []);

            // 修改状态：写入一个用户信息
            await gasComparison.connect(user1).setUserV1(
                user2.address,
                25,                       // age
                ethers.parseEther("100"), // balance
                3,                        // level
                true                      // active
            );

            // 验证：状态确实被改了
            let info = await gasComparison.usersV1(user2.address);
            expect(info.age).to.equal(25);

            // 读档：回滚到快照点
            await ethers.provider.send("evm_revert", [snapshotId]);

            // 验证：状态恢复为默认值（快照时的空状态）
            info = await gasComparison.usersV1(user2.address);
            expect(info.age).to.equal(0);       // 回到默认 uint
            expect(info.balance).to.equal(0n);  // 回到默认 uint256
        });
    });

    // ==================== B 组 — 时间操控 ====================
    describe("B. 时间操控 — 拨快 EVM 时钟", function () {
        it("B1. networkHelpers.time.increase 前进时间（现代写法）", async function () {
            const before = (await ethers.provider.getBlock("latest"))!.timestamp;

            // 为什么用 networkHelpers 而不是底层 RPC？
            // → Hardhat 3 把常用测试操作封装成了语义化 API，可读性更好
            //    time.increase 内部会"挖一个新块"，新块时间戳 = 旧块 + 秒数
            await networkHelpers.time.increase(3600); // +1 小时

            const after = (await ethers.provider.getBlock("latest"))!.timestamp;
            // 为什么用 >= 而不是 ==？
            // → 时间前进的精确值受 EVM 实现细节影响，断言"至少前进了这么多"更稳健
            expect(after).to.be.gte(before + 3600);
        });

        it("B2. 底层 RPC 写法（evm_increaseTime + evm_mine）", async function () {
            const before = (await ethers.provider.getBlock("latest"))!.timestamp;

            // ⚠️ 关键坑：evm_increaseTime 只是"登记"一个时间偏移，
            //    必须再 evm_mine 挖一个块，新块才会用上这个偏移
            //    只 increase 不 mine，block.timestamp 根本不变
            await ethers.provider.send("evm_increaseTime", [60]); // +60 秒
            await ethers.provider.send("evm_mine", []);           // 挖块让偏移生效

            const after = (await ethers.provider.getBlock("latest"))!.timestamp;
            expect(after).to.be.gte(before + 60);
        });
    });

    // ==================== C 组 — 账户冒充 ====================
    describe("C. Impersonate — 冒充任意账户", function () {
        it("C1. 冒充地址 + 注资 + 用其签名交易", async function () {
            // 一个主网巨鲸风格的地址（本地网络里它没有私钥、余额为 0）
            const whaleAddress = "0x1111111111111111111111111111111111111111";

            // 为什么 impersonateAccount？
            // → 测试里常需要扮演"某个特定地址"（巨鲸、受害者、攻击者），
            //    但我们没有它的私钥，impersonate 让 EDR 允许我们用这个地址签名
            await networkHelpers.impersonateAccount(whaleAddress);

            // 为什么 setBalance？
            // → 被冒充的地址余额为 0，没钱付 gas，发不了交易
            //    先给它注入 ETH 当 gas 费（本地测试里"凭空印钱"是合法的）
            await networkHelpers.setBalance(whaleAddress, ethers.parseEther("10"));

            // 为什么 getSigner 而不是 getSigners()[0]？
            // → getSigner(addr) 返回"用该地址签名"的 signer，
            //    前提是该地址已被 impersonate（或本来就是本地 signer）
            const whale = await ethers.getSigner(whaleAddress);

            // 验证 signer 地址正确
            expect(await whale.getAddress()).to.equal(whaleAddress);

            // 用冒充账户真正发一笔交易：转 1 ETH 给 user1
            // 为什么这样做能"证明冒充成功"？
            // → 只有 EDR 认可 whaleAddress 的签名，这笔交易才能上链
            await whale.sendTransaction({
                to: user1.address,
                value: ethers.parseEther("1"),
            });

            // 验证：whale 余额减少（转了 1 ETH + 付了 gas，必然 < 9 ETH）
            const whaleBalanceAfter = await ethers.provider.getBalance(whaleAddress);
            expect(whaleBalanceAfter).to.be.lt(ethers.parseEther("9"));
        });
    });
});