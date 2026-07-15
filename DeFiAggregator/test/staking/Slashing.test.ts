import { expect } from "chai";
import { network } from "hardhat";

const { ethers, networkHelpers } = await network.create();

const MAX_BPS = 10000n; // 100%

enum ViolationType {
    DoubleSigning = 0,
    Downtime = 1,
    MaliciousVote = 2,
}

describe("⚔️ Slashing — 罚没合约测试", function () {
    let slashing: any;
    let owner: any, reporter: any, violator: any;

    const APPEAL_PERIOD = 1 * 24 * 3600; // 1 天申诉期

    // 默认罚没比例（来自合约构造函数）
    const DOUBLE_SIGN_RATE = 5000n; // 50%
    const DOWNTIME_RATE = 500n;     // 5%
    const MALICIOUS_VOTE_RATE = 3000n; // 30%

    beforeEach(async function () {
        [owner, reporter, violator] = await ethers.getSigners();

        // 部署 Slashing 合约
        // stakingContract 先用 owner 地址占位（测试中不实际调用 staking）
        const Slashing = await ethers.getContractFactory("Slashing");
        slashing = await Slashing.deploy(owner.address, APPEAL_PERIOD);
    });

    // ==================== 部署测试 ====================

    describe("A. 部署", function () {
        it("A1. 应该正确设置申诉期", async function () {
            expect(await slashing.appealPeriod()).to.equal(APPEAL_PERIOD);
        });

        it("A2. 默认罚没比例正确", async function () {
            expect(await slashing.slashRate(ViolationType.DoubleSigning))
                .to.equal(DOUBLE_SIGN_RATE);
            expect(await slashing.slashRate(ViolationType.Downtime))
                .to.equal(DOWNTIME_RATE);
            expect(await slashing.slashRate(ViolationType.MaliciousVote))
                .to.equal(MALICIOUS_VOTE_RATE);
        });

        it("A3. 拒绝申诉期过长", async function () {
            const Slashing = await ethers.getContractFactory("Slashing");
            await expect(
                Slashing.deploy(owner.address, 8 * 24 * 3600) // 8 天 > 7 天上限
            ).to.be.revertedWith("Appeal period too long");
        });
    });

    // ==================== 举报违规 ====================

    describe("B. 举报违规 (reportViolation)", function () {
        const stakedAmount = ethers.parseEther("1000");
        const evidence = ethers.keccak256(ethers.toUtf8Bytes("double-sign-evidence"));

        it("B1. 任何人可以举报违规", async function () {
            await expect(
                slashing.connect(reporter).reportViolation(
                    violator.address,
                    ViolationType.DoubleSigning,
                    stakedAmount,
                    evidence
                )
            ).to.emit(slashing, "ViolationReported");
        });

        it("B2. 不同违规类型罚没金额不同", async function () {
            // 双重签名 → 50%
            const ev1 = ethers.keccak256(ethers.toUtf8Bytes("evidence-1"));
            const tx1 = await slashing.connect(reporter).reportViolation(
                violator.address,
                ViolationType.DoubleSigning,
                stakedAmount,
                ev1
            );
            const r1 = await tx1.wait();
            const b1 = await ethers.provider.getBlock(r1.blockNumber);
            const id1 = ethers.keccak256(
                ethers.solidityPacked(
                    ["address", "uint8", "bytes32", "uint256"],
                    [violator.address, ViolationType.DoubleSigning, ev1, b1!.timestamp]
                )
            );

            // 离线 → 5%（用不同的 evidence 产生不同 ID）
            const ev2 = ethers.keccak256(ethers.toUtf8Bytes("evidence-2"));
            const tx2 = await slashing.connect(reporter).reportViolation(
                violator.address,
                ViolationType.Downtime,
                stakedAmount,
                ev2
            );
            const r2 = await tx2.wait();
            const b2 = await ethers.provider.getBlock(r2.blockNumber);
            const id2 = ethers.keccak256(
                ethers.solidityPacked(
                    ["address", "uint8", "bytes32", "uint256"],
                    [violator.address, ViolationType.Downtime, ev2, b2!.timestamp]
                )
            );

            const v1 = await slashing.getViolation(id1);
            const v2 = await slashing.getViolation(id2);

            // 双重签名: 1000 * 50% = 500
            // 离线: 1000 * 5% = 50
            expect(v1.slashAmount).to.equal(stakedAmount * DOUBLE_SIGN_RATE / MAX_BPS);
            expect(v2.slashAmount).to.equal(stakedAmount * DOWNTIME_RATE / MAX_BPS);
            expect(v1.slashAmount).to.not.equal(v2.slashAmount);
        });

        it("B3. 举报后设置 readyTime (当前时间 + 申诉期)", async function () {
            const ev = ethers.keccak256(ethers.toUtf8Bytes("test-ready"));

            const tx = await slashing.connect(reporter).reportViolation(
                violator.address,
                ViolationType.MaliciousVote,
                stakedAmount,
                ev
            );

            // 从事件中获取 id（通过日志解析）
            const receipt = await tx.wait();
            // 手动计算 id
            const block = await ethers.provider.getBlock(receipt.blockNumber);
            const id = ethers.keccak256(
                ethers.solidityPacked(
                    ["address", "uint8", "bytes32", "uint256"],
                    [violator.address, ViolationType.MaliciousVote, ev, block!.timestamp]
                )
            );

            const v = await slashing.getViolation(id);
            expect(v.readyTime).to.equal(block!.timestamp + APPEAL_PERIOD);
            expect(v.executed).to.be.false;
            expect(v.appealed).to.be.false;
        });

        it("B4. 拒绝质押量为 0 的举报", async function () {
            await expect(
                slashing.connect(reporter).reportViolation(
                    violator.address,
                    ViolationType.DoubleSigning,
                    0,
                    evidence
                )
            ).to.be.revertedWith("Zero staked amount");
        });

        it("B5. 拒绝零地址的举报", async function () {
            await expect(
                slashing.connect(reporter).reportViolation(
                    ethers.ZeroAddress,
                    ViolationType.DoubleSigning,
                    stakedAmount,
                    evidence
                )
            ).to.be.revertedWith("Zero address");
        });
    });

    // ==================== 执行罚没 ====================

    describe("C. 执行罚没 (executeSlash)", function () {
        const stakedAmount = ethers.parseEther("1000");

        it("C1. 申诉期过后可以执行罚没", async function () {
            const ev = ethers.keccak256(ethers.toUtf8Bytes("exec-test"));
            const tx = await slashing.connect(reporter).reportViolation(
                violator.address,
                ViolationType.DoubleSigning,
                stakedAmount,
                ev
            );
            const receipt = await tx.wait();
            const block = await ethers.provider.getBlock(receipt.blockNumber);
            const id = ethers.keccak256(
                ethers.solidityPacked(
                    ["address", "uint8", "bytes32", "uint256"],
                    [violator.address, ViolationType.DoubleSigning, ev, block!.timestamp]
                )
            );

            // 时间未到 → 不能执行
            await expect(
                slashing.connect(reporter).executeSlash(id)
            ).to.be.revertedWithCustomError(slashing, "Slashing_NotReady");

            // 快进申诉期
            await networkHelpers.time.increase(APPEAL_PERIOD + 1);

            // 现在可以执行
            await expect(slashing.connect(reporter).executeSlash(id))
                .to.emit(slashing, "Slashed");

            const v = await slashing.getViolation(id);
            expect(v.executed).to.be.true;
            expect(await slashing.slashCount(violator.address)).to.equal(1);
        });

        it("C2. 不能重复执行同一罚没", async function () {
            const ev = ethers.keccak256(ethers.toUtf8Bytes("double-exec"));
            const tx = await slashing.connect(reporter).reportViolation(
                violator.address,
                ViolationType.Downtime,
                stakedAmount,
                ev
            );
            const receipt = await tx.wait();
            const block = await ethers.provider.getBlock(receipt.blockNumber);
            const id = ethers.keccak256(
                ethers.solidityPacked(
                    ["address", "uint8", "bytes32", "uint256"],
                    [violator.address, ViolationType.Downtime, ev, block!.timestamp]
                )
            );

            await networkHelpers.time.increase(APPEAL_PERIOD + 1);
            await slashing.connect(reporter).executeSlash(id);

            await expect(
                slashing.connect(reporter).executeSlash(id)
            ).to.be.revertedWithCustomError(slashing, "Slashing_AlreadyExecuted");
        });

        it("C3. 罚没后 totalSlashed 累加", async function () {
            const ev = ethers.keccak256(ethers.toUtf8Bytes("total-test"));
            const tx = await slashing.connect(reporter).reportViolation(
                violator.address,
                ViolationType.MaliciousVote,
                stakedAmount,
                ev
            );
            const receipt = await tx.wait();
            const block = await ethers.provider.getBlock(receipt.blockNumber);
            const id = ethers.keccak256(
                ethers.solidityPacked(
                    ["address", "uint8", "bytes32", "uint256"],
                    [violator.address, ViolationType.MaliciousVote, ev, block!.timestamp]
                )
            );

            const totalBefore = await slashing.totalSlashed();

            await networkHelpers.time.increase(APPEAL_PERIOD + 1);
            await slashing.connect(reporter).executeSlash(id);

            const totalAfter = await slashing.totalSlashed();
            // 1000 * 30% = 300
            expect(totalAfter - totalBefore).to.equal(
                stakedAmount * MALICIOUS_VOTE_RATE / MAX_BPS
            );
        });
    });

    // ==================== 申诉 ====================

    describe("D. 申诉 (appeal)", function () {
        const stakedAmount = ethers.parseEther("1000");

        it("D1. owner 可以申诉取消罚没", async function () {
            const ev = ethers.keccak256(ethers.toUtf8Bytes("appeal-test"));
            const tx = await slashing.connect(reporter).reportViolation(
                violator.address,
                ViolationType.DoubleSigning,
                stakedAmount,
                ev
            );
            const receipt = await tx.wait();
            const block = await ethers.provider.getBlock(receipt.blockNumber);
            const id = ethers.keccak256(
                ethers.solidityPacked(
                    ["address", "uint8", "bytes32", "uint256"],
                    [violator.address, ViolationType.DoubleSigning, ev, block!.timestamp]
                )
            );

            await slashing.appeal(id);

            const v = await slashing.getViolation(id);
            expect(v.appealed).to.be.true;
        });

        it("D2. 申诉成功后的罚没不能执行", async function () {
            const ev = ethers.keccak256(ethers.toUtf8Bytes("appeal-block"));
            const tx = await slashing.connect(reporter).reportViolation(
                violator.address,
                ViolationType.DoubleSigning,
                stakedAmount,
                ev
            );
            const receipt = await tx.wait();
            const block = await ethers.provider.getBlock(receipt.blockNumber);
            const id = ethers.keccak256(
                ethers.solidityPacked(
                    ["address", "uint8", "bytes32", "uint256"],
                    [violator.address, ViolationType.DoubleSigning, ev, block!.timestamp]
                )
            );

            // Owner 申诉通过
            await slashing.appeal(id);

            await networkHelpers.time.increase(APPEAL_PERIOD + 1);

            // 即使时间到了也不能执行
            await expect(
                slashing.connect(reporter).executeSlash(id)
            ).to.be.revertedWithCustomError(slashing, "Slashing_AlreadyAppealed");
        });

        it("D3. 非 owner 不能申诉", async function () {
            const ev = ethers.keccak256(ethers.toUtf8Bytes("no-auth-appeal"));
            const tx = await slashing.connect(reporter).reportViolation(
                violator.address,
                ViolationType.Downtime,
                stakedAmount,
                ev
            );
            const receipt = await tx.wait();
            const block = await ethers.provider.getBlock(receipt.blockNumber);
            const id = ethers.keccak256(
                ethers.solidityPacked(
                    ["address", "uint8", "bytes32", "uint256"],
                    [violator.address, ViolationType.Downtime, ev, block!.timestamp]
                )
            );

            await expect(
                slashing.connect(reporter).appeal(id)
            ).to.be.revertedWithCustomError(slashing, "OwnableUnauthorizedAccount");
        });
        
        it("D4. 已执行的罚没不能申诉", async function () {
            const ev = ethers.keccak256(ethers.toUtf8Bytes("post-exec-appeal"));
            const tx = await slashing.connect(reporter).reportViolation(
                violator.address,
                ViolationType.Downtime,
                stakedAmount,
                ev
            );
            const receipt = await tx.wait();
            const block = await ethers.provider.getBlock(receipt.blockNumber);
            const id = ethers.keccak256(
                ethers.solidityPacked(
                    ["address", "uint8", "bytes32", "uint256"],
                    [violator.address, ViolationType.Downtime, ev, block!.timestamp]
                )
            );

            await networkHelpers.time.increase(APPEAL_PERIOD + 1);
            await slashing.connect(reporter).executeSlash(id);

            await expect(
                slashing.appeal(id)
            ).to.be.revertedWithCustomError(slashing, "Slashing_AlreadyExecuted");
        });
    });

    // ==================== 管理员 ====================

    describe("E. 管理员配置", function () {
        it("E1. 可以修改罚没比例", async function () {
            const newRate = 6000n; // 60%
            await slashing.setSlashRate(ViolationType.DoubleSigning, newRate);
            expect(await slashing.slashRate(ViolationType.DoubleSigning)).to.equal(newRate);
        });

        it("E2. 罚没比例不能超过 100%", async function () {
            await expect(
                slashing.setSlashRate(ViolationType.DoubleSigning, 10001)
            ).to.be.revertedWithCustomError(slashing, "Slashing_InvalidRate");
        });

        it("E3. 非 owner 不能修改比例", async function () {
            await expect(
                slashing.connect(reporter).setSlashRate(ViolationType.Downtime, 1000)
            ).to.be.revertedWithCustomError(slashing, "OwnableUnauthorizedAccount");
        });

        it("E4. 可以修改申诉期", async function () {
            const newPeriod = 2 * 24 * 3600;
            await slashing.setAppealPeriod(newPeriod);
            expect(await slashing.appealPeriod()).to.equal(newPeriod);
        });
    });

    // ==================== 边界 & 面试场景 ====================

    describe("F. 面试场景", function () {
        const stakedAmount = ethers.parseEther("3200"); // 模拟 32 ETH

        it("F1. 双重签名 → 罚没 50%", async function () {
            const ev = ethers.keccak256(ethers.toUtf8Bytes("interview-double"));
            const tx = await slashing.connect(reporter).reportViolation(
                violator.address,
                ViolationType.DoubleSigning,
                stakedAmount,
                ev
            );
            const receipt = await tx.wait();
            const block = await ethers.provider.getBlock(receipt.blockNumber);
            const id = ethers.keccak256(
                ethers.solidityPacked(
                    ["address", "uint8", "bytes32", "uint256"],
                    [violator.address, ViolationType.DoubleSigning, ev, block!.timestamp]
                )
            );

            const v = await slashing.getViolation(id);
            // 3200 * 50% = 1600
            expect(v.slashAmount).to.equal(stakedAmount * DOUBLE_SIGN_RATE / MAX_BPS);
        });

        it("F2. 离线 → 罚没 5%（轻微）", async function () {
            const ev = ethers.keccak256(ethers.toUtf8Bytes("interview-downtime"));
            const tx = await slashing.connect(reporter).reportViolation(
                violator.address,
                ViolationType.Downtime,
                stakedAmount,
                ev
            );
            const receipt = await tx.wait();
            const block = await ethers.provider.getBlock(receipt.blockNumber);
            const id = ethers.keccak256(
                ethers.solidityPacked(
                    ["address", "uint8", "bytes32", "uint256"],
                    [violator.address, ViolationType.Downtime, ev, block!.timestamp]
                )
            );

            const v = await slashing.getViolation(id);
            // 3200 * 5% = 160
            expect(v.slashAmount).to.equal(stakedAmount * DOWNTIME_RATE / MAX_BPS);
        });

        it("F3. 多次违规 → slashCount 累加", async function () {
            // 第一次违规
            const ev1 = ethers.keccak256(ethers.toUtf8Bytes("multi-1"));
            const tx1 = await slashing.connect(reporter).reportViolation(
                violator.address,
                ViolationType.Downtime,
                stakedAmount,
                ev1
            );
            const r1 = await tx1.wait();
            const b1 = await ethers.provider.getBlock(r1.blockNumber);
            const id1 = ethers.keccak256(
                ethers.solidityPacked(
                    ["address", "uint8", "bytes32", "uint256"],
                    [violator.address, ViolationType.Downtime, ev1, b1!.timestamp]
                )
            );

            await networkHelpers.time.increase(APPEAL_PERIOD + 1);
            await slashing.connect(reporter).executeSlash(id1);
            expect(await slashing.slashCount(violator.address)).to.equal(1);

            // 第二次违规
            const ev2 = ethers.keccak256(ethers.toUtf8Bytes("multi-2"));
            const tx2 = await slashing.connect(reporter).reportViolation(
                violator.address,
                ViolationType.MaliciousVote,
                stakedAmount,
                ev2
            );
            const r2 = await tx2.wait();
            const b2 = await ethers.provider.getBlock(r2.blockNumber);
            const id2 = ethers.keccak256(
                ethers.solidityPacked(
                    ["address", "uint8", "bytes32", "uint256"],
                    [violator.address, ViolationType.MaliciousVote, ev2, b2!.timestamp]
                )
            );

            await networkHelpers.time.increase(APPEAL_PERIOD + 1);
            await slashing.connect(reporter).executeSlash(id2);
            expect(await slashing.slashCount(violator.address)).to.equal(2);
        });
    });
});