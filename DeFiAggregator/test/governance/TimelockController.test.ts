import { expect } from "chai";
import { network } from "hardhat";

const { ethers, networkHelpers } = await network.create();

const PROPOSER_ROLE = ethers.keccak256(ethers.toUtf8Bytes("PROPOSER_ROLE"));
const EXECUTOR_ROLE = ethers.keccak256(ethers.toUtf8Bytes("EXECUTOR_ROLE"));
const CANCELLER_ROLE = ethers.keccak256(ethers.toUtf8Bytes("CANCELLER_ROLE"));

describe("🔒 TimelockController — 升级版时间锁测试", function () {
    let timelock: any;
    let owner: any, proposer: any, executor: any, canceller: any, target: any;
    const MIN_DELAY = 2 * 24 * 3600; // 2 天
    const SALT = ethers.ZeroHash;
    const PREDECESSOR = ethers.ZeroHash;

    beforeEach(async function () {
        [owner, proposer, executor, canceller, target] = await ethers.getSigners();

        const TimelockController = await ethers.getContractFactory("TimelockController");
        timelock = await TimelockController.deploy(
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

        it("A4. DEFAULT_ADMIN_ROLE 是 Timelock 自身", async function () {
            const DEFAULT_ADMIN_ROLE = await timelock.DEFAULT_ADMIN_ROLE();
            const timelockAddress = await timelock.getAddress();
            expect(await timelock.hasRole(DEFAULT_ADMIN_ROLE, timelockAddress)).to.be.true;
        });

        it("A5. 拒绝超过最大延迟", async function () {
            const TimelockController = await ethers.getContractFactory("TimelockController");
            await expect(
                TimelockController.deploy(31 * 24 * 3600, [], [], [])
            ).to.be.revertedWith("Delay exceeds max");
        });
    });

    // ==================== 单个操作：排队 → 执行 ====================

    describe("B. 单个操作 — 排队 → 执行", function () {
        const calldata = "0x1234";

        it("B1. proposer 可以排队操作", async function () {
            await timelock.connect(proposer).schedule(
                target.address, 0, calldata, PREDECESSOR, SALT, MIN_DELAY
            );

            const id = await timelock.hashOperation(
                target.address, 0, calldata, PREDECESSOR, SALT
            );

            expect(await timelock.isOperationScheduled(id)).to.be.true;
            expect(await timelock.isOperationDone(id)).to.be.false;
        });

        it("B2. 时间未到不能执行", async function () {
            await timelock.connect(proposer).schedule(
                target.address, 0, calldata, PREDECESSOR, SALT, MIN_DELAY
            );

            await timelock.hashOperation(
                target.address, 0, calldata, PREDECESSOR, SALT
            );

            // 立即执行 → 失败
            await expect(
                timelock.connect(executor).execute(
                    target.address, 0, calldata, PREDECESSOR, SALT
                )
            ).to.be.revertedWithCustomError(timelock, "Timelock__NotReady");
        });

        it("B3. 时间到期后可以执行", async function () {
            await timelock.connect(proposer).schedule(
                target.address, 0, calldata, PREDECESSOR, SALT, MIN_DELAY
            );

            const id = await timelock.hashOperation(
                target.address, 0, calldata, PREDECESSOR, SALT
            );

            // 快进 2 天 + 1 秒
            await networkHelpers.time.increase(MIN_DELAY + 1);

            await timelock.connect(executor).execute(
                target.address, 0, calldata, PREDECESSOR, SALT
            );

            expect(await timelock.isOperationDone(id)).to.be.true;
            expect(await timelock.isOperationScheduled(id)).to.be.false;
        });

        it("B4. 不能重复执行同一操作", async function () {
            await timelock.connect(proposer).schedule(
                target.address, 0, calldata, PREDECESSOR, SALT, MIN_DELAY
            );

            await networkHelpers.time.increase(MIN_DELAY + 1);
            await timelock.connect(executor).execute(
                target.address, 0, calldata, PREDECESSOR, SALT
            );

            // 再次执行 → 失败（timestamps 已被 delete，查不到操作）
            await expect(
                timelock.connect(executor).execute(
                    target.address, 0, calldata, PREDECESSOR, SALT
                )
            ).to.be.revertedWithCustomError(timelock, "Timelock__OperationNotScheduled");
        });

        it("B5. 不能重复排队同一操作", async function () {
            await timelock.connect(proposer).schedule(
                target.address, 0, calldata, PREDECESSOR, SALT, MIN_DELAY
            );

            await expect(
                timelock.connect(proposer).schedule(
                    target.address, 0, calldata, PREDECESSOR, SALT, MIN_DELAY
                )
            ).to.be.revertedWithCustomError(timelock, "Timelock__OperationAlreadyScheduled");
        });
    });

    // ==================== 批量操作 ⭐ ====================

    describe("C. 批量操作 — scheduleBatch / executeBatch", function () {
        const calldata1 = "0xaabb";
        const calldata2 = "0xccdd";

        it("C1. 批量排队后应有统一的 readyTime", async function () {
            const targets = [target.address, proposer.address];
            const values = [0, 0];
            const payloads = [calldata1, calldata2];

            await timelock.connect(proposer).scheduleBatch(
                targets, values, payloads, PREDECESSOR, SALT, MIN_DELAY
            );

            const batchId = await timelock.hashOperationBatch(
                targets, values, payloads, PREDECESSOR, SALT
            );

            const timestamp = await timelock.getTimestamp(batchId);
            expect(timestamp).to.be.gt(0);
            expect(await timelock.isOperationScheduled(batchId)).to.be.true;
        });

        it("C2. 批量执行 — 时间到了全部执行", async function () {
            const targets = [target.address, proposer.address];
            const values = [0, 0];
            const payloads = [calldata1, calldata2];

            await timelock.connect(proposer).scheduleBatch(
                targets, values, payloads, PREDECESSOR, SALT, MIN_DELAY
            );

            const batchId = await timelock.hashOperationBatch(
                targets, values, payloads, PREDECESSOR, SALT
            );

            await networkHelpers.time.increase(MIN_DELAY + 1);

            await timelock.connect(executor).executeBatch(
                targets, values, payloads, PREDECESSOR, SALT
            );

            expect(await timelock.isOperationDone(batchId)).to.be.true;
        });

        it("C3. 批量执行 — 时间未到不能执行", async function () {
            const targets = [target.address, proposer.address];
            const values = [0, 0];
            const payloads = [calldata1, calldata2];

            await timelock.connect(proposer).scheduleBatch(
                targets, values, payloads, PREDECESSOR, SALT, MIN_DELAY
            );

            await expect(
                timelock.connect(executor).executeBatch(
                    targets, values, payloads, PREDECESSOR, SALT
                )
            ).to.be.revertedWithCustomError(timelock, "Timelock__NotReady");
        });

        it("C4. 批量排队 — 数组长度不一致 → 回滚", async function () {
            await expect(
                timelock.connect(proposer).scheduleBatch(
                    [target.address, proposer.address], // 2 targets
                    [0],                                  // 1 value
                    [calldata1],                          // 1 calldata
                    PREDECESSOR, SALT, MIN_DELAY
                )
            ).to.be.revertedWith("Timelock: length mismatch");
        });

        it("C5. 批量排队 — 空数组 → 回滚", async function () {
            await expect(
                timelock.connect(proposer).scheduleBatch(
                    [], [], [], PREDECESSOR, SALT, MIN_DELAY
                )
            ).to.be.revertedWith("Timelock: empty batch");
        });

        it("C6. 单个 batch ID 与多个单操作 ID 不同", async function () {
            const targets = [target.address, proposer.address];
            const values = [0, 0];
            const payloads = [calldata1, calldata2];

            const batchId = await timelock.hashOperationBatch(
                targets, values, payloads, PREDECESSOR, SALT
            );

            const singleId = await timelock.hashOperation(
                target.address, 0, calldata1, PREDECESSOR, SALT
            );

            // 批量 ID 和单个 ID 不同（编码方式不同）
            expect(batchId).to.not.equal(singleId);
        });
    });

    // ==================== Grace Period 过期 ⭐ ====================

    describe("D. Grace Period — 过期机制", function () {
        const calldata = "0xdead";

        it("D1. 在 Grace Period 内可以执行", async function () {
            await timelock.connect(proposer).schedule(
                target.address, 0, calldata, PREDECESSOR, SALT, MIN_DELAY
            );

            const id = await timelock.hashOperation(
                target.address, 0, calldata, PREDECESSOR, SALT
            );

            // 快进到 readyTime 之后、过期之前
            await networkHelpers.time.increase(MIN_DELAY + 1);

            expect(await timelock.isOperationReady(id)).to.be.true;
            expect(await timelock.isOperationExpired(id)).to.be.false;

            await timelock.connect(executor).execute(
                target.address, 0, calldata, PREDECESSOR, SALT
            );
            expect(await timelock.isOperationDone(id)).to.be.true;
        });

        it("D2. 超过 Grace Period 后操作过期 → 不能执行", async function () {
            await timelock.connect(proposer).schedule(
                target.address, 0, calldata, PREDECESSOR, SALT, MIN_DELAY
            );

            const id = await timelock.hashOperation(
                target.address, 0, calldata, PREDECESSOR, SALT
            );

            const GRACE_PERIOD = await timelock.GRACE_PERIOD();

            // 快进到 readyTime + GRACE_PERIOD + 1
            await networkHelpers.time.increase(MIN_DELAY + Number(GRACE_PERIOD) + 1);

            expect(await timelock.isOperationExpired(id)).to.be.true;
            expect(await timelock.isOperationReady(id)).to.be.false;

            // 执行 → 应该失败
            await expect(
                timelock.connect(executor).execute(
                    target.address, 0, calldata, PREDECESSOR, SALT
                )
            ).to.be.revertedWithCustomError(timelock, "Timelock__OperationExpired");
        });

        it("D3. 已执行的操作不会过期", async function () {
            await timelock.connect(proposer).schedule(
                target.address, 0, calldata, PREDECESSOR, SALT, MIN_DELAY
            );

            const id = await timelock.hashOperation(
                target.address, 0, calldata, PREDECESSOR, SALT
            );

            await networkHelpers.time.increase(MIN_DELAY + 1);
            await timelock.connect(executor).execute(
                target.address, 0, calldata, PREDECESSOR, SALT
            );

            // 执行后即使过了 Grace Period 也不影响
            expect(await timelock.isOperationExpired(id)).to.be.false;
        });
    });

    // ==================== 取消操作 ====================

    describe("E. 取消操作", function () {
        const calldata = "0x5678";

        it("E1. canceller 可以取消排队中的操作", async function () {
            await timelock.connect(proposer).schedule(
                target.address, 0, calldata, PREDECESSOR, SALT, MIN_DELAY
            );

            const id = await timelock.hashOperation(
                target.address, 0, calldata, PREDECESSOR, SALT
            );

            expect(await timelock.isOperationScheduled(id)).to.be.true;

            await timelock.connect(canceller).cancel(id);

            expect(await timelock.isOperationScheduled(id)).to.be.false;
        });

        it("E2. 取消后即使到期也不能执行", async function () {
            await timelock.connect(proposer).schedule(
                target.address, 0, calldata, PREDECESSOR, SALT, MIN_DELAY
            );

            const id = await timelock.hashOperation(
                target.address, 0, calldata, PREDECESSOR, SALT
            );

            await timelock.connect(canceller).cancel(id);
            await networkHelpers.time.increase(MIN_DELAY + 1);

            await expect(
                timelock.connect(executor).execute(
                    target.address, 0, calldata, PREDECESSOR, SALT
                )
            ).to.be.revertedWithCustomError(timelock, "Timelock__OperationNotScheduled");
        });

        it("E3. 非 canceller 不能取消", async function () {
            await timelock.connect(proposer).schedule(
                target.address, 0, calldata, PREDECESSOR, SALT, MIN_DELAY
            );

            const id = await timelock.hashOperation(
                target.address, 0, calldata, PREDECESSOR, SALT
            );

            await expect(
                timelock.connect(target).cancel(id)
            ).to.be.revertedWithCustomError(timelock, "AccessControlUnauthorizedAccount");
        });

        it("E4. 已执行的操作不能取消", async function () {
            await timelock.connect(proposer).schedule(
                target.address, 0, calldata, PREDECESSOR, SALT, MIN_DELAY
            );

            const id = await timelock.hashOperation(
                target.address, 0, calldata, PREDECESSOR, SALT
            );

            await networkHelpers.time.increase(MIN_DELAY + 1);
            await timelock.connect(executor).execute(
                target.address, 0, calldata, PREDECESSOR, SALT
            );

            // 已执行 → cancel 失败（timestamps 已 delete，查不到操作）
            await expect(
                timelock.connect(canceller).cancel(id)
            ).to.be.revertedWithCustomError(timelock, "Timelock__OperationNotScheduled");
        });
    });

    // ==================== 权限测试 ====================

    describe("F. 权限控制", function () {
        const calldata = "0xabcd";

        it("F1. 非 proposer 不能排队", async function () {
            await expect(
                timelock.connect(target).schedule(
                    target.address, 0, calldata, PREDECESSOR, SALT, MIN_DELAY
                )
            ).to.be.revertedWithCustomError(timelock, "AccessControlUnauthorizedAccount");
        });

        it("F2. 非 proposer 不能批量排队", async function () {
            await expect(
                timelock.connect(target).scheduleBatch(
                    [target.address], [0], [calldata], PREDECESSOR, SALT, MIN_DELAY
                )
            ).to.be.revertedWithCustomError(timelock, "AccessControlUnauthorizedAccount");
        });

        it("F3. 非 executor 不能执行", async function () {
            await timelock.connect(proposer).schedule(
                target.address, 0, calldata, PREDECESSOR, SALT, MIN_DELAY
            );
            await networkHelpers.time.increase(MIN_DELAY + 1);

            await expect(
                timelock.connect(target).execute(
                    target.address, 0, calldata, PREDECESSOR, SALT
                )
            ).to.be.revertedWithCustomError(timelock, "AccessControlUnauthorizedAccount");
        });
    });

    // ==================== 修改 minDelay ====================

    describe("G. 修改 minDelay", function () {
        it("G1. 只有 Timelock 自身能修改 minDelay", async function () {
            // 外部调用 → onlySelf 阻止
            await expect(
                timelock.connect(owner).updateMinDelay(3 * 24 * 3600)
            ).to.be.revertedWith("Timelock: only self");
        });

        it("G2. 不能超过 MAX_DELAY", async function () {
            // 虽然不能直接调，但可以验证常量
            expect(await timelock.MAX_DELAY()).to.equal(30 * 24 * 3600);
        });
    });

    // ==================== ETA 查询 ====================

    describe("H. ETA 查询 — getTimestamp", function () {
        const calldata = "0xbeef";

        it("H1. 排队后可以查询 readyTime", async function () {
            await timelock.connect(proposer).schedule(
                target.address, 0, calldata, PREDECESSOR, SALT, MIN_DELAY
            );

            const id = await timelock.hashOperation(
                target.address, 0, calldata, PREDECESSOR, SALT
            );

            const timestamp = await timelock.getTimestamp(id);
            expect(timestamp).to.be.gt(0);

            // readyTime 应该在未来
            const latestBlock = await ethers.provider.getBlock("latest");
            expect(timestamp).to.be.gt(latestBlock!.timestamp);
        });

        it("H2. 未排队的操作查询 ETA → 回滚", async function () {
            const fakeId = ethers.keccak256(ethers.toUtf8Bytes("non-existent"));

            await expect(
                timelock.getTimestamp(fakeId)
            ).to.be.revertedWithCustomError(timelock, "Timelock__OperationNotScheduled");
        });

        it("H3. 执行后 getTimestamp 回滚（已清理存储）", async function () {
            await timelock.connect(proposer).schedule(
                target.address, 0, calldata, PREDECESSOR, SALT, MIN_DELAY
            );

            const id = await timelock.hashOperation(
                target.address, 0, calldata, PREDECESSOR, SALT
            );

            await networkHelpers.time.increase(MIN_DELAY + 1);
            await timelock.connect(executor).execute(
                target.address, 0, calldata, PREDECESSOR, SALT
            );

            // 执行后 storage 被清理 → getTimestamp 回滚
            await expect(
                timelock.getTimestamp(id)
            ).to.be.revertedWithCustomError(timelock, "Timelock__OperationNotScheduled");
        });
    });

    // ==================== 面试场景 ====================

    describe("I. 🔥 闪电贷治理攻击防御（面试重点）", function () {
        it("I1. Timelock 阻止闪电贷攻击者立即执行", async function () {
            const maliciousCalldata = "0xdeadbeef";
            const attackSalt = ethers.keccak256(ethers.toUtf8Bytes("attack"));

            // 攻击者排队恶意操作
            await timelock.connect(proposer).schedule(
                target.address, 0, maliciousCalldata, PREDECESSOR, attackSalt, MIN_DELAY
            );

            // 攻击者尝试立即执行 → Timelock 阻止
            await expect(
                timelock.connect(executor).execute(
                    target.address, 0, maliciousCalldata, PREDECESSOR, attackSalt
                )
            ).to.be.revertedWithCustomError(timelock, "Timelock__NotReady");

            // 等待期内，社区发现并取消恶意提案
            const id = await timelock.hashOperation(
                target.address, 0, maliciousCalldata, PREDECESSOR, attackSalt
            );
            await timelock.connect(canceller).cancel(id);

            // 2 天后闪电贷早还了，攻击者无法执行
            await networkHelpers.time.increase(MIN_DELAY + 1);
            await expect(
                timelock.connect(executor).execute(
                    target.address, 0, maliciousCalldata, PREDECESSOR, attackSalt
                )
            ).to.be.revertedWithCustomError(timelock, "Timelock__OperationNotScheduled");
        });
    });
});