import { expect } from "chai";
import { network } from "hardhat";

const { ethers, networkHelpers } = await network.create();

const PROPOSER_ROLE = ethers.keccak256(ethers.toUtf8Bytes("PROPOSER_ROLE"));
const EXECUTOR_ROLE = ethers.keccak256(ethers.toUtf8Bytes("EXECUTOR_ROLE"));
const CANCELLER_ROLE = ethers.keccak256(ethers.toUtf8Bytes("CANCELLER_ROLE"));

describe("🔒 Timelock — 时间锁合约测试", function () {
    let timelock: any;
    let owner: any, proposer: any, executor: any, canceller: any, target: any;
    const MIN_DELAY = 2 * 24 * 3600; // 2 天

    beforeEach(async function () {
        [owner, proposer, executor, canceller, target] = await ethers.getSigners();

        const Timelock = await ethers.getContractFactory("Timelock");
        timelock = await Timelock.deploy(
            MIN_DELAY,
            [proposer.address],
            [executor.address],
            [canceller.address]
        );
    });

    // ==================== 部署测试 ====================

    describe("A. 部署", function () {
        it("A1. 应该正确设置 minDelay", async function () {
            expect(await timelock.minDelay()).to.equal(MIN_DELAY);
        });

        it("A2. 应该正确分配角色", async function () {
            expect(await timelock.hasRole(PROPOSER_ROLE, proposer.address)).to.be.true;
            expect(await timelock.hasRole(EXECUTOR_ROLE, executor.address)).to.be.true;
            expect(await timelock.hasRole(CANCELLER_ROLE, canceller.address)).to.be.true;
        });

        it("A3. 非授权地址不应有角色", async function () {
            expect(await timelock.hasRole(PROPOSER_ROLE, target.address)).to.be.false;
        });

        it("A4. 拒绝超过最大延迟", async function () {
            const Timelock = await ethers.getContractFactory("Timelock");
            await expect(
                Timelock.deploy(31 * 24 * 3600, [], [], [])
            ).to.be.revertedWith("Delay exceeds max");
        });
    });

    // ==================== 正常流程 ====================

    describe("B. 排队 → 执行（正常流程）", function () {
        const calldata = "0x1234"; // 模拟 calldata
        const salt = ethers.ZeroHash;
        const predecessor = ethers.ZeroHash;

        it("B1. proposer 可以排队操作", async function () {
            const tx = await timelock.connect(proposer).schedule(
                target.address, 0, calldata, predecessor, salt
            );
            const receipt = await tx.wait();

            // 从事件中提取 id
            const id = await timelock.hashOperation(
                target.address, 0, calldata, predecessor, salt
            );

            expect(await timelock.isOperationSchedule(id)).to.be.true;
            expect(await timelock.isOperationDone(id)).to.be.false;
        });

        it("B2. 时间未到不能执行", async function () {
            const id = await timelock.hashOperation(
                target.address, 0, calldata, predecessor, salt
            );

            await timelock.connect(proposer).schedule(
                target.address, 0, calldata, predecessor, salt
            );

            // 立即执行 → 应该失败
            await expect(
                timelock.connect(executor).execute(
                    target.address, 0, calldata, predecessor, salt
                )
            ).to.be.revertedWithCustomError(timelock, "Timelock_NotReady");
        });

        it("B3. 时间到期后可以执行", async function () {
            const id = await timelock.hashOperation(
                target.address, 0, calldata, predecessor, salt
            );

            await timelock.connect(proposer).schedule(
                target.address, 0, calldata, predecessor, salt
            );

            // 快进 2 天
            await networkHelpers.time.increase(MIN_DELAY + 1);

            await timelock.connect(executor).execute(
                target.address, 0, calldata, predecessor, salt
            );

            expect(await timelock.isOperationDone(id)).to.be.true;
            expect(await timelock.isOperationSchedule(id)).to.be.false;
        });

        it("B4. 不能重复执行同一操作", async function () {
            await timelock.connect(proposer).schedule(
                target.address, 0, calldata, predecessor, salt
            );

            await networkHelpers.time.increase(MIN_DELAY + 1);
            await timelock.connect(executor).execute(
                target.address, 0, calldata, predecessor, salt
            );

            // 再次执行 → 失败
            await expect(
                timelock.connect(executor).execute(
                    target.address, 0, calldata, predecessor, salt
                )
            ).to.be.revertedWithCustomError(timelock, "Timelock_AlreadyDone");
        });
    });

    // ==================== 取消操作 ====================

    describe("C. 取消操作", function () {
        const calldata = "0x5678";
        const salt = ethers.ZeroHash;

        it("C1. canceller 可以取消排队中的操作", async function () {
            const id = await timelock.hashOperation(
                target.address, 0, calldata, ethers.ZeroHash, salt
            );

            await timelock.connect(proposer).schedule(
                target.address, 0, calldata, ethers.ZeroHash, salt
            );

            expect(await timelock.isOperationSchedule(id)).to.be.true;

            await timelock.connect(canceller).cancel(id);

            expect(await timelock.isOperationSchedule(id)).to.be.false;
            expect(await timelock.isOperationDone(id)).to.be.false;
        });

        it("C2. 取消后即使到期也不能执行", async function () {
            const id = await timelock.hashOperation(
                target.address, 0, calldata, ethers.ZeroHash, salt
            );

            await timelock.connect(proposer).schedule(
                target.address, 0, calldata, ethers.ZeroHash, salt
            );

            // 先取消
            await timelock.connect(canceller).cancel(id);

            // 时间到期
            await networkHelpers.time.increase(MIN_DELAY + 1);

            // 尝试执行 → 失败（已取消）
            await expect(
                timelock.connect(executor).execute(
                    target.address, 0, calldata, ethers.ZeroHash, salt
                )
            ).to.be.revertedWithCustomError(timelock, "Timelock_OperationNotScheduled");
        });

        it("C3. 非 canceller 不能取消", async function () {
            await timelock.connect(proposer).schedule(
                target.address, 0, calldata, ethers.ZeroHash, salt
            );

            const id = await timelock.hashOperation(
                target.address, 0, calldata, ethers.ZeroHash, salt
            );

            await expect(
                timelock.connect(target).cancel(id) // target 没有 CANCELLER_ROLE
            ).to.be.revertedWithCustomError(timelock, "AccessControlUnauthorizedAccount");
        });

        it("C4. 已执行的操作不能取消", async function () {
            await timelock.connect(proposer).schedule(
                target.address, 0, calldata, ethers.ZeroHash, salt
            );

            await networkHelpers.time.increase(MIN_DELAY + 1);
            await timelock.connect(executor).execute(
                target.address, 0, calldata, ethers.ZeroHash, salt
            );

            const id = await timelock.hashOperation(
                target.address, 0, calldata, ethers.ZeroHash, salt
            );

            await expect(
                timelock.connect(canceller).cancel(id)
            ).to.be.revertedWithCustomError(timelock, "Timelock_AlreadyDone");
        });
    });

    // ==================== 权限测试 ====================

    describe("D. 权限控制", function () {
        const calldata = "0xabcd";
        const salt = ethers.ZeroHash;

        it("D1. 非 proposer 不能排队", async function () {
            await expect(
                timelock.connect(target).schedule(
                    target.address, 0, calldata, ethers.ZeroHash, salt
                )
            ).to.be.revertedWithCustomError(timelock, "AccessControlUnauthorizedAccount");
        });

        it("D2. 非 executor 不能执行", async function () {
            await timelock.connect(proposer).schedule(
                target.address, 0, calldata, ethers.ZeroHash, salt
            );

            await networkHelpers.time.increase(MIN_DELAY + 1);

            await expect(
                timelock.connect(target).execute(
                    target.address, 0, calldata, ethers.ZeroHash, salt
                )
            ).to.be.revertedWithCustomError(timelock, "AccessControlUnauthorizedAccount");
        });
    });

    // ==================== 不同 salt 产生不同 ID ====================

    describe("E. 操作 ID 唯一性", function () {
        const calldata = "0xdead";

        it("E1. 不同 salt → 不同 ID", async function () {
            const salt1 = ethers.keccak256(ethers.toUtf8Bytes("salt1"));
            const salt2 = ethers.keccak256(ethers.toUtf8Bytes("salt2"));

            await timelock.connect(proposer).schedule(
                target.address, 0, calldata, ethers.ZeroHash, salt1
            );

            await timelock.connect(proposer).schedule(
                target.address, 0, calldata, ethers.ZeroHash, salt2
            );

            const id1 = await timelock.hashOperation(
                target.address, 0, calldata, ethers.ZeroHash, salt1
            );
            const id2 = await timelock.hashOperation(
                target.address, 0, calldata, ethers.ZeroHash, salt2
            );

            expect(id1).to.not.equal(id2);
            expect(await timelock.isOperationSchedule(id1)).to.be.true;
            expect(await timelock.isOperationSchedule(id2)).to.be.true;
        });

        it("E2. 不同 target → 不同 ID", async function () {
            const id1 = await timelock.hashOperation(
                target.address, 0, calldata, ethers.ZeroHash, ethers.ZeroHash
            );
            const id2 = await timelock.hashOperation(
                proposer.address, 0, calldata, ethers.ZeroHash, ethers.ZeroHash
            );

            expect(id1).to.not.equal(id2);
        });

        it("E3. 同一操作不能重复排队", async function () {
            const salt = ethers.ZeroHash;

            await timelock.connect(proposer).schedule(
                target.address, 0, calldata, ethers.ZeroHash, salt
            );

            await expect(
                timelock.connect(proposer).schedule(
                    target.address, 0, calldata, ethers.ZeroHash, salt
                )
            ).to.be.revertedWith("Timelock: already scheduled");
        });
    });

    // ==================== 管理员 ====================

    describe("F. 修改 minDelay", function () {
        it("F1. admin 可以修改 minDelay", async function () {
            const newDelay = 3 * 24 * 3600; // 3 天
            await timelock.updateMinDelay(newDelay);
            expect(await timelock.minDelay()).to.equal(newDelay);
        });

        it("F2. 不能超过 MAX_DELAY", async function () {
            await expect(
                timelock.updateMinDelay(31 * 24 * 3600)
            ).to.be.revertedWith("Delay exceeds max");
        });

        it("F3. 非 admin 不能修改", async function () {
            await expect(
                timelock.connect(proposer).updateMinDelay(3 * 24 * 3600)
            ).to.be.revertedWithCustomError(timelock, "AccessControlUnauthorizedAccount");
        });
    });

    // ==================== 查询操作详情 ====================

    describe("G. 查询函数", function () {
        it("G1. getOperation 返回完整信息", async function () {
            const calldata = "0xbeef";
            const salt = ethers.keccak256(ethers.toUtf8Bytes("query-test"));

            await timelock.connect(proposer).schedule(
                target.address, 0, calldata, ethers.ZeroHash, salt
            );

            const id = await timelock.hashOperation(
                target.address, 0, calldata, ethers.ZeroHash, salt
            );

            const op = await timelock.getOperation(id);

            expect(op.target).to.equal(target.address);
            expect(op.value).to.equal(0);
            expect(op.data).to.equal(calldata);
            expect(op.done).to.be.false;
            expect(op.readyTime).to.be.gt(0);
        });

        it("G2. 未排队的操作返回空数据", async function () {
            const id = ethers.keccak256(ethers.toUtf8Bytes("nonexistent"));
            const op = await timelock.getOperation(id);
            expect(op.readyTime).to.equal(0);
            expect(op.done).to.be.false;
        });
    });

    // ==================== 面试场景 ====================

    describe("H. 🔥 闪电贷治理攻击防御（面试重点）", function () {
        it("H1. Timelock 阻止闪电贷攻击者立即执行", async function () {
            // 模拟恶意 calldata（例如：drainFunds）
            const maliciousCalldata = "0xdeadbeef";
            const salt = ethers.keccak256(ethers.toUtf8Bytes("attack"));

            // 攻击者通过控制 proposer 排队恶意操作
            await timelock.connect(proposer).schedule(
                target.address, 0, maliciousCalldata, ethers.ZeroHash, salt
            );

            // 攻击者尝试立即执行 → Timelock 阻止
            await expect(
                timelock.connect(executor).execute(
                    target.address, 0, maliciousCalldata, ethers.ZeroHash, salt
                )
            ).to.be.revertedWithCustomError(timelock, "Timelock_NotReady");

            // 等待期内，社区发现并取消恶意提案
            const id = await timelock.hashOperation(
                target.address, 0, maliciousCalldata, ethers.ZeroHash, salt
            );
            await timelock.connect(canceller).cancel(id);

            // 2 天后闪电贷早还了，攻击者即使拿到 executor 也没用
            await networkHelpers.time.increase(MIN_DELAY + 1);
            await expect(
                timelock.connect(executor).execute(
                    target.address, 0, maliciousCalldata, ethers.ZeroHash, salt
                )
            ).to.be.revertedWithCustomError(timelock, "Timelock_OperationNotScheduled");
        });
    });

    // ==================== 前置操作（predecessor） ====================

    describe("I. 前置操作链", function () {
        it("I1. 前置操作未完成时不能执行后置操作", async function () {
            const calldata = "0x1111";
            const predecessorSalt = ethers.keccak256(ethers.toUtf8Bytes("pre"));
            const successorSalt = ethers.keccak256(ethers.toUtf8Bytes("post"));

            // 先排队前置操作
            await timelock.connect(proposer).schedule(
                target.address, 0, calldata, ethers.ZeroHash, predecessorSalt
            );
            const predecessorId = await timelock.hashOperation(
                target.address, 0, calldata, ethers.ZeroHash, predecessorSalt
            );

            // 排队后置操作（依赖前置操作）
            await timelock.connect(proposer).schedule(
                target.address, 0, calldata, predecessorId, successorSalt
            );

            await networkHelpers.time.increase(MIN_DELAY + 1);

            // 尝试执行后置操作 → 失败（前置操作未完成）
            await expect(
                timelock.connect(executor).execute(
                    target.address, 0, calldata, predecessorId, successorSalt
                )
            ).to.be.revertedWith("Timelock: predecessor not done");
        });

        it("I2. 前置操作完成后可以执行后置操作", async function () {
            const calldata = "0x2222";
            const preSalt = ethers.keccak256(ethers.toUtf8Bytes("chain-pre"));
            const postSalt = ethers.keccak256(ethers.toUtf8Bytes("chain-post"));

            await timelock.connect(proposer).schedule(
                target.address, 0, calldata, ethers.ZeroHash, preSalt
            );
            const predecessorId = await timelock.hashOperation(
                target.address, 0, calldata, ethers.ZeroHash, preSalt
            );

            await timelock.connect(proposer).schedule(
                target.address, 0, calldata, predecessorId, postSalt
            );

            await networkHelpers.time.increase(MIN_DELAY + 1);

            // 先执行前置
            await timelock.connect(executor).execute(
                target.address, 0, calldata, ethers.ZeroHash, preSalt
            );

            // 再执行后置 → 成功
            await timelock.connect(executor).execute(
                target.address, 0, calldata, predecessorId, postSalt
            );

            const postId = await timelock.hashOperation(
                target.address, 0, calldata, predecessorId, postSalt
            );
            expect(await timelock.isOperationDone(postId)).to.be.true;
        });
    });
});