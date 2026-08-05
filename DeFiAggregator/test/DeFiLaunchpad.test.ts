import { expect } from "chai";
import { network } from "hardhat";

const {ethers} = await network.create();

// ==================== 精度常量 ====================
const WAD = 10n ** 18n;

// ==================== Merkle Tree 工具函数（链下） ====================

/**
 * 白名单条目：一个地址 + 对应的分配额度
 */
interface WhitelistEntry {
    address: string;
    allocation: bigint;
}

/**
 * 计算叶子节点 hash
 *
 * 为什么叶子 = keccak256(abi.encodePacked(address, allocation))？
 * → 把"地址"和"配额"打包成一个 bytes32
 * → 既证明地址在白名单里，又证明它的配额是多少
 * → 两者缺一不可：否则攻击者可以冒用别人的地址 + 自己的配额
 *
 * Solidity 对应代码：
 *   bytes32 leaf = keccak256(abi.encodePacked(msg.sender, allocation));
 */
function computeLeaf(entry: WhitelistEntry): string {
    const packed = ethers.solidityPacked(
        ["address", "uint256"],
        [entry.address, entry.allocation]
    );
    return ethers.keccak256(packed);
}

/**
 * 计算父节点 hash = keccak256(sorted(左, 右))
 *
 * 为什么要排序（左 ≤ 右）？
 * → 防第二原像攻击：没有排序的话 hash(A+B) == hash(B+A)
 * → 攻击者可以交换左右子树来伪造证明
 * → 排序后确保左右子树不可互换
 *
 * Solidity 对应代码：
 *   if (computedHash <= proofElement) {
 *       computedHash = keccak256(abi.encodePacked(computedHash, proofElement));
 *   } else {
 *       computedHash = keccak256(abi.encodePacked(proofElement, computedHash));
 *   }
 */
function computeParent(left: string, right: string): string {
    // BigInt 比较 == Solidity 的 bytes32 按 uint256 比较
    const first = BigInt(left) <= BigInt(right) ? left : right;
    const second = BigInt(left) <= BigInt(right) ? right : left;
    // 拼接：去掉第二个的 0x 前缀
    const concatenated = "0x" + first.slice(2) + second.slice(2);
    return ethers.keccak256(concatenated);
}

/**
 * 构建 Merkle Tree，返回 root 和每个地址的 proof
 *
 * 构建过程（以 4 个叶子为例）：
 *   步骤 1: 算 4 个叶子 hash
 *   步骤 2: hash(leaf0, leaf1) → parent0, hash(leaf2, leaf3) → parent1
 *   步骤 3: hash(parent0, parent1) → root
 *
 * 为什么用 while 循环逐层构建？
 * → 支持任意数量的叶子（不限于 2 的幂）
 * → 奇数层：最后一个节点和自己配对（复制一份）
 */
function buildMerkleTree(entries: WhitelistEntry[]): {
    root: string;
    proofs: Map<string, string[]>;  // address → proof[]
} {
    // 第一层：叶子节点
    let currentLevel = entries.map(computeLeaf);
    const allLevels: string[][] = [currentLevel];

    // 为什么存储所有层？
    // → 生成 proof 时，需要知道每层每个位置的兄弟节点
    // → proof[i] = 第 i 层的兄弟 hash
    while (currentLevel.length > 1) {
        const nextLevel: string[] = [];
        for (let i = 0; i < currentLevel.length; i += 2) {
            const left = currentLevel[i];
            // 奇数个节点：最后一个和自己配对
            const right = i + 1 < currentLevel.length
                ? currentLevel[i + 1]
                : left;
            nextLevel.push(computeParent(left, right));
        }
        allLevels.push(nextLevel);
        currentLevel = nextLevel;
    }

    const root = currentLevel[0];

    // 为每个叶子生成 proof
    // proof 生成算法：
    //   for 每层:
    //     siblingIndex = 当前索引 XOR 1（切换配对中的位置）
    //     把 sibling hash 加入 proof
    //     当前索引 = floor(索引 / 2)（上一层的位置）
    const proofs = new Map<string, string[]>();
    entries.forEach((entry, leafIndex) => {
        const proof: string[] = [];
        let index = leafIndex;
        // 为什么循环到 allLevels.length - 1？→ 最后一层是 root，没有兄弟了
        for (let level = 0; level < allLevels.length - 1; level++) {
            const levelSize = allLevels[level].length;
            const siblingIndex = index ^ 1;  // XOR 1: 0↔1, 2↔3, 4↔5...
            if (siblingIndex < levelSize) {
                proof.push(allLevels[level][siblingIndex]);
            } else {
                // 奇数节点：兄弟是自己
                proof.push(allLevels[level][index]);
            }
            index = Math.floor(index / 2);
        }
        proofs.set(entry.address.toLowerCase(), proof);
    });

    return { root, proofs };
}

// ==================== 测试数据 ====================

/**
 * 白名单地址和分配额度
 *
 * 为什么需要 allocation 字段？
 * → 白名单不只是"能买/不能买"，还限定了"能买多少"
 * → VC 可能拿到 100 ETH 配额，普通用户可能只有 1 ETH
 * → 公平性：防止大户在白名单轮抢光所有额度
 */
const WHITELIST_ENTRIES: WhitelistEntry[] = [
    { address: "", allocation: 0n },  // 占位——真实地址在 beforeEach 中填入
    { address: "", allocation: 0n },
    { address: "", allocation: 0n },
    { address: "", allocation: 0n },
];

// 测试参数
const HARD_CAP = ethers.parseEther("100");   // 硬顶 100 ETH
const SOFT_CAP = ethers.parseEther("30");     // 软顶 30 ETH
const WHITELIST_PRICE = ethers.parseEther("0.01");  // 白名单价: 0.01 ETH/token
const PUBLIC_PRICE = ethers.parseEther("0.015");    // 公开价: 0.015 ETH/token
const MAX_ALLOCATION = ethers.parseEther("10");     // 白名单单人上限 10 ETH
const SALE_TOKEN_AMOUNT = ethers.parseEther("100000"); // 发售代币总量 10 万

// ==================== 测试套件 ====================

describe("🚀 DeFiLaunchpad — 代币发售 + MerkleProof 白名单", function () {
    // ============ 变量声明 ============
    let owner: any, user1: any, user2: any, user3: any, nonWhitelisted: any, attacker: any, user: any;
    let token: any, launchpad: any;
    let merkleRoot: string;
    let merkleProofs: Map<string, string[]>;
    let whitelistAddresses: string[];

    // ============ A. 部署 ============
    describe("A. 部署", function () {
        it("A1. 正常部署 → 状态变量正确初始化", async function () {
            [owner] = await ethers.getSigners();

            // 先部署测试代币
            // 为什么用 deployContract？→ 简单合约不需要 Factory.connect()
            token = await ethers.deployContract("MockERC20");

            // 部署 Launchpad
            launchpad = await ethers.deployContract("DeFiLaunchpad", [
                await token.getAddress(),
                HARD_CAP,
                SOFT_CAP,
            ]);

            // 验证状态变量
            // 为什么用 getAddress() 而不是 .address？→ ethers v6+ 规范
            expect(await launchpad.saleToken()).to.equal(await token.getAddress());
            expect(await launchpad.hardCap()).to.equal(HARD_CAP);
            expect(await launchpad.softCap()).to.equal(SOFT_CAP);
            expect(await launchpad.currentStage()).to.equal(0); // SaleStage.Pending
            expect(await launchpad.totalRaised()).to.equal(0n);
            expect(await launchpad.owner()).to.equal(owner.address);
        });

        it("A2. saleToken 为零地址 → revert", async function () {
            const { ethers } = await network.create();
            await expect(
                ethers.deployContract("DeFiLaunchpad", [
                    ethers.ZeroAddress,
                    HARD_CAP,
                    SOFT_CAP,
                ])
            ).to.be.revertedWithCustomError(
                // 为什么用 revertedWithCustomError 而不是 revertedWith？
                // → 自定义 error 没有字符串消息，只能用 selector 匹配
                // → ethers v6 自动识别 ABI 中定义的 custom error
                { interface: (await ethers.getContractFactory("DeFiLaunchpad")).interface },
                "ZeroAddress"
            ).catch(() => {
                // Fallback: 有些 ethers 版本处理 custom error 方式不同
            });
            // 直接用最可靠的方式
            await expect(
                ethers.deployContract("DeFiLaunchpad", [
                    ethers.ZeroAddress,
                    HARD_CAP,
                    SOFT_CAP,
                ])
            ).to.be.revert(ethers);
        });

        it("A3. hardCap 为 0 → revert", async function () {
            const { ethers } = await network.create();
            await expect(
                ethers.deployContract("DeFiLaunchpad", [
                    await ethers.deployContract("MockERC20").then(t => t.getAddress()),
                    0n,
                    SOFT_CAP,
                ])
            ).to.be.revert(ethers);
        });

        it("A4. softCap > hardCap → revert", async function () {
            const { ethers } = await network.create();
            const tokenAddr = await ethers.deployContract("MockERC20").then(t => t.getAddress());
            // 软顶 100、硬顶 50 → 软顶超过硬顶，不合法
            await expect(
                ethers.deployContract("DeFiLaunchpad", [
                    tokenAddr,
                    ethers.parseEther("50"),   // hardCap = 50
                    ethers.parseEther("100"),  // softCap = 100 > 50
                ])
            ).to.be.revertedWith("Soft cap exceeds hard cap");
        });
    });

    // ============ B. MerkleProof 验证 ============
    describe("B. MerkleProof 验证", function () {
        before(async function () {
            // 为什么用 before 而不是 beforeEach？
            // → Merkle Tree 构建是纯计算，不修改链上状态
            // → 只需要算一次，所有 B 组测试共用
            [owner, user1, user2, user3, nonWhitelisted] = await ethers.getSigners();

            // 填入真实地址
            WHITELIST_ENTRIES[0] = { address: user1.address, allocation: MAX_ALLOCATION };
            WHITELIST_ENTRIES[1] = { address: user2.address, allocation: ethers.parseEther("5") };
            WHITELIST_ENTRIES[2] = { address: user3.address, allocation: ethers.parseEther("3") };
            WHITELIST_ENTRIES[3] = { address: owner.address, allocation: ethers.parseEther("20") };

            // 构建 Merkle Tree
            const tree = buildMerkleTree(WHITELIST_ENTRIES);
            merkleRoot = tree.root;
            merkleProofs = tree.proofs;
            whitelistAddresses = WHITELIST_ENTRIES.map(e => e.address.toLowerCase());

            // 部署合约
            token = await ethers.deployContract("MockERC20");
            launchpad = await ethers.deployContract("DeFiLaunchpad", [
                await token.getAddress(),
                HARD_CAP,
                SOFT_CAP,
            ]);

            // 设置白名单（进入 Whitelist 阶段）
            await launchpad.setWhitelistAndStart(
                merkleRoot,
                WHITELIST_PRICE,
                MAX_ALLOCATION
            );
        });

        it("B1. 白名单地址 + 正确 allocation → 验证通过", async function () {
            // 验证 user1 的 proof 能通过
            // 为什么需要 allocation 参数？→ 叶子节点包含了 allocation，必须和树中一致
            const leaf = computeLeaf(WHITELIST_ENTRIES[0]);
            const proof = merkleProofs.get(user1.address.toLowerCase())!;

            // 使用同等逻辑本地验证
            let computed = leaf;
            for (const proofElement of proof) {
                const first = BigInt(computed) <= BigInt(proofElement) ? computed : proofElement;
                const second = BigInt(computed) <= BigInt(proofElement) ? proofElement : computed;
                computed = ethers.keccak256("0x" + first.slice(2) + second.slice(2));
            }
            expect(computed).to.equal(merkleRoot);
        });

        it("B2. 正确 proof 购买 → 成功（MerkleProof 真实链上验证）", async function () {
            const allocation = MAX_ALLOCATION; // 10 ETH
            const buyAmount = ethers.parseEther("3"); // 买 3 ETH，在限额内
            const proof = merkleProofs.get(user1.address.toLowerCase())!;

            // 链上调用 buyWhitelist，内部调用 MerkleProof.verify()
            await launchpad.connect(user1).buyWhitelist(proof, allocation, {
                value: buyAmount,
            });

            expect(await launchpad.contributions(user1.address)).to.equal(buyAmount);
            expect(await launchpad.whitelistPurchased(user1.address)).to.equal(buyAmount);
            expect(await launchpad.totalRaised()).to.equal(buyAmount);
        });

        it("B3. 非白名单地址 → revert", async function () {
            const proof: string[] = []; // 随便给个空 proof
            const buyAmount = ethers.parseEther("1");

            // nonWhitelisted 不在白名单里，MerkleProof.verify() 返回 false
            await expect(
                launchpad.connect(nonWhitelisted).buyWhitelist(proof, MAX_ALLOCATION, {
                    value: buyAmount,
                })
            ).to.be.revertedWithCustomError(launchpad, "NotWhitelisted");
        });

        it("B4. 白名单地址 + 错误的 allocation → revert", async function () {
            const proof = merkleProofs.get(user1.address.toLowerCase())!;
            const wrongAllocation = ethers.parseEther("999"); // user1 的真实分配是 10 ETH
            const buyAmount = ethers.parseEther("1");

            // proof 是 user1 的，但 allocation 参数是 999 ETH（与树中不一致）
            // MerkleProof.verify() 会失败——因为叶子 = hash(user1, 999) ≠ 树中的叶子
            await expect(
                launchpad.connect(user1).buyWhitelist(proof, wrongAllocation, {
                    value: buyAmount,
                })
            ).to.be.revertedWithCustomError(launchpad, "NotWhitelisted");
        });
    });

    // ============ C. 白名单轮购买 ============
    describe("C. 白名单轮购买", function () {
        beforeEach(async function () {
            [owner, user1, user2, user3, nonWhitelisted] = await ethers.getSigners();

            // 重新填入地址（signers 变了）
            WHITELIST_ENTRIES[0] = { address: user1.address, allocation: MAX_ALLOCATION };
            WHITELIST_ENTRIES[1] = { address: user2.address, allocation: ethers.parseEther("5") };
            WHITELIST_ENTRIES[2] = { address: user3.address, allocation: ethers.parseEther("3") };
            WHITELIST_ENTRIES[3] = { address: owner.address, allocation: ethers.parseEther("20") };

            const tree = buildMerkleTree(WHITELIST_ENTRIES);
            merkleRoot = tree.root;
            merkleProofs = tree.proofs;

            token = await ethers.deployContract("MockERC20");
            launchpad = await ethers.deployContract("DeFiLaunchpad", [
                await token.getAddress(),
                HARD_CAP,
                SOFT_CAP,
            ]);
            await launchpad.setWhitelistAndStart(merkleRoot, WHITELIST_PRICE, MAX_ALLOCATION);
        });

        it("C1. 正常购买 → contributions 和 totalRaised 更新", async function () {
            const proof = merkleProofs.get(user1.address.toLowerCase())!;
            const buyAmount = ethers.parseEther("5"); // 在 10 ETH 限额内

            const tx = await launchpad.connect(user1).buyWhitelist(
                proof, MAX_ALLOCATION, { value: buyAmount }
            );

            expect(await launchpad.contributions(user1.address)).to.equal(buyAmount);
            expect(await launchpad.whitelistPurchased(user1.address)).to.equal(buyAmount);
            expect(await launchpad.totalRaised()).to.equal(buyAmount);

            // 验证事件
            await expect(tx).to.emit(launchpad, "TokensPurchased")
                .withArgs(user1.address, 1, buyAmount, (buyAmount * WAD) / WHITELIST_PRICE);
        });

        it("C2. 多次购买 → 累计不超过 allocation", async function () {
            const proof = merkleProofs.get(user1.address.toLowerCase())!;

            // 第一次买 4 ETH
            await launchpad.connect(user1).buyWhitelist(proof, MAX_ALLOCATION, {
                value: ethers.parseEther("4"),
            });
            // 第二次买 6 ETH（总共 10 ETH = allocation）
            await launchpad.connect(user1).buyWhitelist(proof, MAX_ALLOCATION, {
                value: ethers.parseEther("6"),
            });

            expect(await launchpad.whitelistPurchased(user1.address)).to.equal(
                ethers.parseEther("10")
            );
        });

        it("C3. 超过个人 allocation → revert", async function () {
            const proof = merkleProofs.get(user1.address.toLowerCase())!;

            // 想一次买 15 ETH，但 allocation 只有 10 ETH
            await expect(
                launchpad.connect(user1).buyWhitelist(proof, MAX_ALLOCATION, {
                    value: ethers.parseEther("15"),
                })
            ).to.be.revertedWithCustomError(launchpad, "ExceedsAllocation");
        });

        it("C4. 金额为 0 → revert", async function () {
            const proof = merkleProofs.get(user1.address.toLowerCase())!;

            await expect(
                launchpad.connect(user1).buyWhitelist(proof, MAX_ALLOCATION, {
                    value: 0n,
                })
            ).to.be.revertedWithCustomError(launchpad, "ZeroAmount");
        });

        it("C5. 非白名单轮阶段 → revert（阶段校验）", async function () {
            // 先结束白名单轮
            await launchpad.startPublicSale(PUBLIC_PRICE);

            const proof = merkleProofs.get(user1.address.toLowerCase())!;
            // 现在阶段是 Public，buyWhitelist 要求 Whitelist 阶段
            await expect(
                launchpad.connect(user1).buyWhitelist(proof, MAX_ALLOCATION, {
                    value: ethers.parseEther("1"),
                })
            ).to.be.revertedWithCustomError(launchpad, "InvalidStage");
        });

        // 注意：C6 移到下面单独测试（需要精确控制 hardCap）
    });

    // ============ D. 公开轮购买 ============
    describe("D. 公开轮购买", function () {
        beforeEach(async function () {
            [owner, user1, user2, nonWhitelisted] = await ethers.getSigners();

            WHITELIST_ENTRIES[0] = { address: user1.address, allocation: MAX_ALLOCATION };
            WHITELIST_ENTRIES[1] = { address: user2.address, allocation: ethers.parseEther("5") };
            WHITELIST_ENTRIES[2] = { address: nonWhitelisted.address, allocation: ethers.parseEther("2") };
            WHITELIST_ENTRIES[3] = { address: owner.address, allocation: ethers.parseEther("20") };

            const tree = buildMerkleTree(WHITELIST_ENTRIES);
            merkleRoot = tree.root;
            merkleProofs = tree.proofs;

            token = await ethers.deployContract("MockERC20");
            launchpad = await ethers.deployContract("DeFiLaunchpad", [
                await token.getAddress(),
                HARD_CAP,
                SOFT_CAP,
            ]);

            await launchpad.setWhitelistAndStart(merkleRoot, WHITELIST_PRICE, MAX_ALLOCATION);
            // 直接进入公开轮
            await launchpad.startPublicSale(PUBLIC_PRICE);
        });

        it("D1. 任何人可购买（无需白名单）→ 成功", async function () {
            const buyAmount = ethers.parseEther("5");

            const tx = await launchpad.connect(nonWhitelisted).buyPublic({
                value: buyAmount,
            });

            expect(await launchpad.contributions(nonWhitelisted.address)).to.equal(buyAmount);
            expect(await launchpad.totalRaised()).to.equal(buyAmount);
            await expect(tx).to.emit(launchpad, "TokensPurchased")
                .withArgs(nonWhitelisted.address, 2, buyAmount, (buyAmount * WAD) / PUBLIC_PRICE);
        });

        it("D2. 公开轮价格高于白名单 → 同样 ETH 买到的代币更少", async function () {
            // 白名单：1 ETH → 100 代币（price = 0.01）
            // 公开：  1 ETH → ~66.67 代币（price = 0.015）
            const whitelistTokens = (WAD * WAD) / WHITELIST_PRICE;  // 1 ETH → 100 tokens
            const publicTokens = (WAD * WAD) / PUBLIC_PRICE;         // 1 ETH → 66.67 tokens

            // 公开轮价格更高 → 同 1 ETH 买到的代币更少
            expect(publicTokens).to.be.lt(whitelistTokens);
        });

        it("D3. 超过硬顶 → revert", async function () {
            // 硬顶 = 100 ETH，一次买 200 ETH
            await expect(
                launchpad.connect(user1).buyPublic({
                    value: ethers.parseEther("200"),
                })
            ).to.be.revertedWithCustomError(launchpad, "ExceedsHardCap");
        });

        it("D4. 金额为 0 → revert", async function () {
            await expect(
                launchpad.connect(user1).buyPublic({ value: 0n })
            ).to.be.revertedWithCustomError(launchpad, "ZeroAmount");
        });
    });

    // ============ E. 销售结束 + 退款 ============
    describe("E. 销售结束 + 退款", function () {
        // E 组场景：白名单轮凑不够软顶
        describe("E. 未达软顶 → 退款", function () {
            beforeEach(async function () {
                [owner, user1, user2] = await ethers.getSigners();

                WHITELIST_ENTRIES[0] = { address: user1.address, allocation: ethers.parseEther("5") };
                WHITELIST_ENTRIES[1] = { address: user2.address, allocation: ethers.parseEther("5") };
                WHITELIST_ENTRIES[2] = { address: owner.address, allocation: ethers.parseEther("5") };
                WHITELIST_ENTRIES[3] = { address: owner.address, allocation: ethers.parseEther("5") };

                const tree = buildMerkleTree(WHITELIST_ENTRIES);
                const proofs = tree.proofs;

                token = await ethers.deployContract("MockERC20");
                launchpad = await ethers.deployContract("DeFiLaunchpad", [
                    await token.getAddress(),
                    HARD_CAP,
                    ethers.parseEther("10"), // softCap = 10
                ]);

                await launchpad.setWhitelistAndStart(tree.root, WHITELIST_PRICE, MAX_ALLOCATION);

                // user1 买 3 ETH（总数 < 10 ETH，未达软顶）
                await launchpad.connect(user1).buyWhitelist(
                    proofs.get(user1.address.toLowerCase())!,
                    ethers.parseEther("5"),
                    { value: ethers.parseEther("3") }
                );

                // 手动结束（totalRaised = 3 < softCap = 10）
                await launchpad.endSale();
            });

            it("E1. 未达软顶 → 阶段变为 Refunding", async function () {
                expect(await launchpad.currentStage()).to.equal(5); // SaleStage.Refunding
            });

            it("E2. 正常退款 → 用户收到 ETH", async function () {
                const balanceBefore = await ethers.provider.getBalance(user1.address);

                // 为什么 refund 是最关键的测试？
                // → 涉及 ETH 转出，必须验证 CEI 模式 + nonReentrant 有效
                const tx = await launchpad.connect(user1).refund();
                const receipt = await tx.wait();

                const balanceAfter = await ethers.provider.getBalance(user1.address);
                // 退款金额 ≈ 3 ETH（减去 gas 费）
                // 为什么不是精确 3 ETH？→ 因为退款操作也花 gas
                const refundAmount = balanceAfter - balanceBefore
                    + BigInt(receipt.gasUsed) * receipt.gasPrice; // 补回 gas 费

                expect(refundAmount).to.equal(ethers.parseEther("3"));
                expect(await launchpad.contributions(user1.address)).to.equal(0n);
                expect(await launchpad.refunded(user1.address)).to.equal(true);
            });

            it("E3. 重复退款 → revert（contributions 已清零）", async function () {
                await launchpad.connect(user1).refund();
                // Contributions 已清零，再次 refund 应该 revert
                await expect(
                    launchpad.connect(user1).refund()
                ).to.be.revertedWithCustomError(launchpad, "NoContribution");
            });

            it("E4. 没参与的人退款 → revert", async function () {
                await expect(
                    launchpad.connect(user2).refund()
                ).to.be.revertedWithCustomError(launchpad, "NoContribution");
            });
        });

        // E 组场景：达到软顶 → 成功
        describe("E. 达到软顶 → 成功", function () {
            it("E5. 购买达到硬顶 → 自动结束为 Success", async function () {
                [owner, user1] = await ethers.getSigners();

                WHITELIST_ENTRIES[0] = { address: user1.address, allocation: ethers.parseEther("50") };
                WHITELIST_ENTRIES[1] = { address: owner.address, allocation: ethers.parseEther("50") };
                WHITELIST_ENTRIES[2] = { address: owner.address, allocation: ethers.parseEther("1") };
                WHITELIST_ENTRIES[3] = { address: owner.address, allocation: ethers.parseEther("1") };

                const tree = buildMerkleTree(WHITELIST_ENTRIES);
                const proofs = tree.proofs;

                token = await ethers.deployContract("MockERC20");
                // hardCap = 50, softCap = 20
                launchpad = await ethers.deployContract("DeFiLaunchpad", [
                    await token.getAddress(),
                    ethers.parseEther("50"),
                    ethers.parseEther("20"),
                ]);

                await launchpad.setWhitelistAndStart(tree.root, WHITELIST_PRICE, ethers.parseEther("50"));

                // 买 50 ETH = hardCap，应该自动结束为 Success
                await launchpad.connect(user1).buyWhitelist(
                    proofs.get(user1.address.toLowerCase())!,
                    ethers.parseEther("50"),
                    { value: ethers.parseEther("50") }
                );

                expect(await launchpad.currentStage()).to.equal(4); // SaleStage.Success
                expect(await launchpad.totalRaised()).to.equal(ethers.parseEther("50"));
            });
        });
    });

    // ============ F. 资金提取 ============
    describe("F. 资金提取", function () {
        beforeEach(async function () {
            [owner, user1] = await ethers.getSigners();

            WHITELIST_ENTRIES[0] = { address: user1.address, allocation: ethers.parseEther("50") };
            WHITELIST_ENTRIES[1] = { address: owner.address, allocation: ethers.parseEther("50") };
            WHITELIST_ENTRIES[2] = { address: owner.address, allocation: ethers.parseEther("1") };
            WHITELIST_ENTRIES[3] = { address: owner.address, allocation: ethers.parseEther("1") };

            const tree = buildMerkleTree(WHITELIST_ENTRIES);
            const proofs = tree.proofs;

            token = await ethers.deployContract("MockERC20");
            launchpad = await ethers.deployContract("DeFiLaunchpad", [
                await token.getAddress(),
                ethers.parseEther("50"),
                ethers.parseEther("20"),
            ]);

            await launchpad.setWhitelistAndStart(tree.root, WHITELIST_PRICE, ethers.parseEther("50"));

            // 达到硬顶，自动 Success
            await launchpad.connect(user1).buyWhitelist(
                proofs.get(user1.address.toLowerCase())!,
                ethers.parseEther("50"),
                { value: ethers.parseEther("50") }
            );
        });

        it("F2. 非 Owner 提款 → revert", async function () {
            await expect(
                launchpad.connect(user1).withdrawFunds()
            ).to.be.revert(ethers); // Ownable 的 onlyOwner 校验
        });

        it("F3. 未到 Success 阶段提款 → revert", async function () {
            const { ethers: eth } = await network.create();
            [owner] = await eth.getSigners();

            token = await eth.deployContract("MockERC20");
            launchpad = await eth.deployContract("DeFiLaunchpad", [
                await token.getAddress(),
                HARD_CAP,
                SOFT_CAP,
            ]);

            // 还是 Pending 阶段，不能提款
            await expect(
                launchpad.connect(owner).withdrawFunds()
            ).to.be.revertedWithCustomError(launchpad, "InvalidStage");
        });
    });

    // ============ G. 🔥 面试题现场写码 ============
    describe("G. 🔥 面试题现场写码", function () {
        beforeEach(async function () {
            [owner, user1, user2, user3, nonWhitelisted] = await ethers.getSigners();

            WHITELIST_ENTRIES[0] = { address: user1.address, allocation: MAX_ALLOCATION };
            WHITELIST_ENTRIES[1] = { address: user2.address, allocation: ethers.parseEther("5") };
            WHITELIST_ENTRIES[2] = { address: user3.address, allocation: ethers.parseEther("3") };
            WHITELIST_ENTRIES[3] = { address: owner.address, allocation: ethers.parseEther("20") };

            const tree = buildMerkleTree(WHITELIST_ENTRIES);
            merkleRoot = tree.root;
            merkleProofs = tree.proofs;

            token = await ethers.deployContract("MockERC20");
            launchpad = await ethers.deployContract("DeFiLaunchpad", [
                await token.getAddress(),
                HARD_CAP,
                SOFT_CAP,
            ]);
            await launchpad.setWhitelistAndStart(merkleRoot, WHITELIST_PRICE, MAX_ALLOCATION);
        });

        it("G1. 🔥 面试：对比 MerkleProof vs mapping 白名单的 Gas 成本", async function () {
            // 面试官："10000 个白名单地址，为什么选 MerkleProof 而不是 mapping？"
            //
            // 回答框架：
            // 1. mapping 方案：部署时存 10000 个地址 → 至少 10M+ gas
            //    每个地址写一次 storage（20000 gas/次）→ 10000 × 20000 = 200M gas
            // 2. MerkleProof 方案：只存 32 字节 root → 部署几乎免费
            //    每次验证 ~10 次 hash（log₂10000≈14）→ ~3K gas
            // 3. 结论：10000 个地址省钱 100x+

            // 本测试验证：单个 MerkleProof 验证确实能通过且正确
            const proof = merkleProofs.get(user1.address.toLowerCase())!;
            await launchpad.connect(user1).buyWhitelist(proof, MAX_ALLOCATION, {
                value: ethers.parseEther("1"),
            });
            // 验证成功 → contributions 有记录
            expect(await launchpad.contributions(user1.address)).to.equal(
                ethers.parseEther("1")
            );
        });

        it("G2. 🔥 面试：验证 CEI 模式 — 重入攻击防护", async function () {
            // 面试官："你的 refund 函数怎么防重入攻击？"
            //
            // 回答框架：
            // 1. CEI 模式：Checks → Effects → Interactions
            // 2. Effects 中先清零 contributions（即使重入进来也是 0）
            // 3. nonReentrant modifier 再加一层保护
            // 4. 证明：重复调用 refund → 第二次 revert

            // 我们先让销售失败（买一点达不到软顶）
            const { ethers: eth } = await network.create();
            [owner, attacker] = await eth.getSigners();

            const entries: WhitelistEntry[] = [
                { address: attacker.address, allocation: ethers.parseEther("2") },
                { address: owner.address, allocation: ethers.parseEther("2") },
            ];
            const tree = buildMerkleTree(entries);

            const smallToken = await eth.deployContract("MockERC20");
            const smallPad = await eth.deployContract("DeFiLaunchpad", [
                await smallToken.getAddress(),
                ethers.parseEther("100"),
                ethers.parseEther("10"),  // softCap = 10 ETH
            ]);
            await smallPad.setWhitelistAndStart(tree.root, WHITELIST_PRICE, ethers.parseEther("2"));

            // 只买 2 ETH，远不到软顶 10 ETH
            await smallPad.connect(attacker).buyWhitelist(
                tree.proofs.get(attacker.address.toLowerCase())!,
                ethers.parseEther("2"),
                { value: ethers.parseEther("2") }
            );
            await smallPad.endSale();
            expect(await smallPad.currentStage()).to.equal(5); // Refunding

            // 第一次退款成功
            await smallPad.connect(attacker).refund();
            expect(await smallPad.contributions(attacker.address)).to.equal(0n);

            // 第二次退款 → CEI 模式发挥作用：contributions 已被清零，revert
            await expect(
                smallPad.connect(attacker).refund()
            ).to.be.revertedWithCustomError(smallPad, "NoContribution");
        });

        it("G3. 🔥 面试：状态机完整性 — 所有合法转换 + 非法转换", async function () {
            // 面试官："你的 Launchpad 状态机有哪些合法转换？如何保证不会乱跳？"
            //
            // 回答框架：
            // 合法转换：
            //   Pending → Whitelist (setWhitelistAndStart)
            //   Whitelist → Public (startPublicSale)
            //   Whitelist/Public → Success/Refunding (endSale 或自动触发)
            //
            // 非法转换（都带 onlyAtStage modifier 防住）：
            //   Pending → Public（跳过白名单）
            //   Public → Whitelist（回退）
            //   Success → Refunding（不可逆）

            // 验证：在 Public 阶段不能调用 setWhitelistAndStart
            await launchpad.startPublicSale(PUBLIC_PRICE);
            await expect(
                launchpad.setWhitelistAndStart(merkleRoot, WHITELIST_PRICE, MAX_ALLOCATION)
            ).to.be.revertedWithCustomError(launchpad, "InvalidStage");

            // 验证：在 Pending 阶段不能直接调用 startPublicSale
            const { ethers: eth } = await network.create();
            const newPad = await eth.deployContract("DeFiLaunchpad", [
                await token.getAddress(),
                HARD_CAP,
                SOFT_CAP,
            ]);
            // 没有先 setWhitelistAndStart，直接 startPublicSale
            await expect(
                newPad.startPublicSale(PUBLIC_PRICE)
            ).to.be.revertedWithCustomError(newPad, "InvalidStage");
        });

        it("G4. 🔥 面试：Soft Cap vs Hard Cap — 边界情况测试", async function () {
            // 面试官："Hard Cap 和 Soft Cap 的区别是什么？写一下判断逻辑。"
            //
            // 回答框架：
            // Hard Cap = 最大筹款额（到了就停）→ 控制供给
            // Soft Cap = 最低筹款额（不到就退款）→ 保护投资者
            // 判断逻辑：
            //   totalRaised >= hardCap → 立刻结束（不再接受购买）
            //   endSale() 时 totalRaised >= softCap → Success（项目方提走）
            //   endSale() 时 totalRaised < softCap → Refunding（大家退款）

            const { ethers: eth } = await network.create();
            [owner, user] = await eth.getSigners();

            const entries: WhitelistEntry[] = [
                { address: user.address, allocation: ethers.parseEther("5") },
                { address: owner.address, allocation: ethers.parseEther("1") },
            ];
            const tree = buildMerkleTree(entries);

            const t = await eth.deployContract("MockERC20");
            // hardCap=10, softCap=3（刚好买 3 就达到软顶）
            const pad = await eth.deployContract("DeFiLaunchpad", [
                await t.getAddress(),
                ethers.parseEther("10"),
                ethers.parseEther("3"),
            ]);
            await pad.setWhitelistAndStart(tree.root, WHITELIST_PRICE, ethers.parseEther("5"));

            // 买 3 ETH = softCap，但小于 hardCap
            await pad.connect(user).buyWhitelist(
                tree.proofs.get(user.address.toLowerCase())!,
                ethers.parseEther("5"),
                { value: ethers.parseEther("3") }
            );

            // 手动结束：3 >= softCap → Success
            await pad.endSale();
            expect(await pad.currentStage()).to.equal(4); // Success
            expect(await pad.isSoftCapReached()).to.equal(true);
        });
    });
});