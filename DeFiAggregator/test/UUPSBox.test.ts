import { expect } from "chai";
import { network } from "hardhat";

const { ethers } = await network.create()

// ==================== 测试常量 ====================
const INITIAL_VALUE = 42n;

describe("🔄 UUPSBox — 可升级合约代理模式", function () {
    // ==================== 变量声明 ====================
    let owner: any, attacker: any, user1: any;
    let v1Logic: any;         // UUPSBoxV1 逻辑合约
    let v2Logic: any;         // UUPSBoxV2 逻辑合约
    let proxy: any;            // UUPSProxy (= ERC1967Proxy wrapper)
    let box: any;              // 通过代理访问的 UUPSBoxV1
    let v2LogicNew: any;       // F 组测试用的第二个 V2 部署

    // ==================== 部署 ====================
    describe("A. 部署", function () {
        it("A1. 应该成功部署 V1 逻辑合约", async function () {
            [owner, attacker, user1] = await ethers.getSigners();

            // 部署 V1 逻辑合约
            v1Logic = await ethers.deployContract("UUPSBoxV1");

            // 验证 constant 版本号（逻辑合约自身的版本号）
            expect(await v1Logic.VERSION()).to.equal("1.0.0");

            // ⚠️ 直接调用 value 会失败——因为逻辑合约未通过代理初始化
            await expect(v1Logic.value()).to.revert;
        });

        it("A2. 应该成功部署 ERC1967 代理并指向 V1", async function () {
            // 准备初始化数据：调用 initialize(INITIAL_VALUE)
            const initData = v1Logic.interface.encodeFunctionData("initialize", [INITIAL_VALUE,]);

            // 部署 UUPSProxy（基于 ERC1967Proxy）
            // 参数：(逻辑合约地址, 初始化数据)
            proxy = await ethers.deployContract("UUPSProxy", [
                await v1Logic.getAddress(),
                initData,
            ]);

            // 用 V1 的 ABI 连接代理合约
            box = await ethers.getContractAt("UUPSBoxV1", await proxy.getAddress());
        });

        it("A3. 代理应该正确初始化并返回初始值", async function () {
            // ⭐ 关键验证：存储通过代理正确初始化了
            expect(await box.value()).to.equal(INITIAL_VALUE);
        });
    });

    // ==================== V1 功能测试 ====================
    describe("B. V1 基础功能", function () {
        it("B1. 应该成功递增", async function () {
            const oldValue = await box.value();

            await box.connect(owner).increment(10n);

            expect(await box.value()).to.equal(oldValue + 10n);
        });

        it("B2. 应该正确触发事件", async function () {
            await expect(box.connect(owner).increment(5n))
                .to.emit(box, "ValueChanged")
                .withArgs(52n, 57n);  // oldValue=52, newValue=57

            await expect(box.connect(owner).increment(5n))
                .to.emit(box, "Incremented")
                .withArgs(5n);
        });

        it("B3. 非 owner 也可以调用 increment（无权限控制）", async function () {
            await box.connect(user1).increment(1n);
            expect(await box.value()).to.equal(63n);
        });

        it("B4. 🔥 非 owner 尝试升级 → revert", async function () {
            // ⭐ 核心安全验证：只有 owner 能升级
            await expect(
                box.connect(attacker).upgradeToAndCall(ethers.ZeroAddress, "0x")
            ).to.be.revertedWithCustomError(box, "OwnableUnauthorizedAccount");
        });

        it("B5. 🔥 V1 不支持 decrement → 调用会失败", async function () {
            // V1 没有 decrement 函数，所以调用必然失败
            // 这就是需要升级的原因！
            // decrement 不在 V1 的 ABI 中，用底层 call 验证
            const data = ethers.id("decrement(uint256)").slice(0, 10)
                + ethers.AbiCoder.defaultAbiCoder().encode(["uint256"], [1n]).slice(2);
            await expect(
                owner.sendTransaction({ to: await box.getAddress(), data })
            ).to.revert(ethers);
        });
    });

    // ==================== 升级到 V2 ====================
    describe("C. 升级流程", function () {
        it("C1. 应该成功部署 V2 逻辑合约", async function () {
            v2Logic = await ethers.deployContract("UUPSBoxV2");
            expect(await v2Logic.VERSION()).to.equal("2.0.0");
        });

        it("C2. owner 应该成功升级代理到 V2", async function () {
            // ⭐ 升级操作：调用 V1 的 upgradeToAndCall（通过代理）
            const tx = await box.connect(owner).upgradeToAndCall(
                await v2Logic.getAddress(),
                "0x"
            );

            // 验证事件
            await expect(tx)
                .to.emit(box, "Upgraded")
                .withArgs(await v2Logic.getAddress());

            // 用 V2 的 ABI 重新连接代理
            box = await ethers.getContractAt("UUPSBoxV2", await proxy.getAddress());
        });

        it("C3. 🔥 核心验证：升级后存储不丢失", async function () {
            // ⭐⭐⭐ 这是代理模式最重要的测试！
            // 升级前 value = 63（经过 B 组测试后）
            // 升级后 value 应该保持不变
            expect(await box.value()).to.equal(63n);

             // VERSION constant 应该变成 2.0.0
            expect(await box.VERSION()).to.equal("2.0.0");
        });
  });

  // ==================== V2 新功能测试 ====================
  describe("D. V2 新功能", function () {
    it("D1. 递增功能仍然正常", async function () {
      const oldValue = await box.value();

      await box.connect(owner).increment(10n);

      expect(await box.value()).to.equal(oldValue + 10n);
      // V2 的事件多了 version 字段
      await expect(box.connect(owner).increment(5n))
        .to.emit(box, "ValueChanged")
        .withArgs(73n, 78n, "2.0.0");
    });

    it("D2. 🆕 递减功能正常工作", async function () {
      const valueBefore = await box.value();

      await box.connect(owner).decrement(3n);

      expect(await box.value()).to.equal(valueBefore - 3n);
    });

    it("D3. 递减应该触发 Decremented 事件", async function () {
      await expect(box.connect(owner).decrement(2n))
        .to.emit(box, "Decremented")
        .withArgs(2n);
    });

    it("D4. 递减超过余额 → revert", async function () {
      const currentValue = await box.value();
      await expect(
        box.connect(user1).decrement(currentValue + 1n)
      ).to.be.revertedWith("Underflow: insufficient value");
    });

    it("D5. 🆕 getStats 返回完整统计信息", async function () {
      const stats = await box.getStats();
      expect(stats.currentValue).to.equal(73n);  // 63+10+5-3-2=73
      expect(stats.version).to.equal("2.0.0");
      // netChange: V1 increments did not update netChang; only V2 ops count: +10+5-3-2=10
      expect(stats.totalNetChange).to.equal(10n);
      expect(stats.lastDecrementTime).to.be.gt(0n);
    });
  });

  // ==================== 安全测试 ====================
  describe("E. 安全边界", function () {
    it("E1. 非 owner 不能升级（升级后仍然有效）", async function () {
      await expect(
        box.connect(attacker).upgradeToAndCall(ethers.ZeroAddress, "0x")
      ).to.be.revertedWithCustomError(box, "OwnableUnauthorizedAccount");
    });

    it("E2. 🔥 逻辑合约本身不能被直接初始化", async function () {
      // 验证 _disableInitializers() 生效
      await expect(
        v1Logic.initialize(999n)
      ).to.be.revertedWithCustomError(v1Logic, "InvalidInitialization");
    });

    it("E3. 🔥 直接调用逻辑合约（绕过代理）不会影响代理存储", async function () {
      // 这是关键安全点：
      // 如果不小心直接调用了逻辑合约而不是代理，会发生什么？
      const v2Direct = await ethers.getContractAt(
        "UUPSBoxV2",
        await v2Logic.getAddress()
      );

      // 直接调用逻辑合约的 increment——它修改的是逻辑合约自己的存储
      // 不会影响代理的存储！
      try {
        await v2Direct.increment(1000n);
        // 逻辑合约可能没有被代理初始化，所以直接调用会失败
      } catch {
        // 预期：直接调用逻辑合约的 increment 会失败
      }

      // 代理的 value 不应该受影响
      expect(await box.value()).to.equal(73n);
    });
  });

  // ==================== 面试演示测试 ====================
  describe("F. 🔥 面试重点", function () {
    it("F1. 🔥 再次升级验证 — 存储持久性多次升级", async function () {
      // 如果再部署一个 V2 来升级，存储仍应保持
      v2LogicNew = await ethers.deployContract("UUPSBoxV2");

      const valueBefore = await box.value();

      await box.connect(owner).upgradeToAndCall(await v2LogicNew.getAddress(), "0x");

      // 用新的逻辑合约 ABI 重新连接
      box = await ethers.getContractAt("UUPSBoxV2", await proxy.getAddress());

      // 存储不变！
      expect(await box.value()).to.equal(valueBefore);
      expect(await box.VERSION()).to.equal("2.0.0");
    });

    it("F2. 🔥 delegatecall 验证 — 代理地址是 msg.sender 的调用者", async function () {
      // 证明：通过代理调用时，存储修改的是代理的存储
      // 而不是逻辑合约的存储
      const proxyAddress = await proxy.getAddress();
      const v2Address = await v2Logic.getAddress();

      // 代理和逻辑合约是不同的地址
      expect(proxyAddress).to.not.equal(v2Address);

      // 通过代理修改 value
      await box.connect(owner).increment(1n);

      // value 被修改在代理的存储中
      const newValue = await box.value();
      expect(newValue).to.be.gt(0n);
    });

    it("F3. 🔥 验证 ERC1967 存储槽", async function () {
      // 直接读取代理合约的实现地址存储槽
      const IMPLEMENTATION_SLOT =
        "0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc";

      const implAddress = await ethers.provider.getStorage(
        await proxy.getAddress(),
        IMPLEMENTATION_SLOT
      );

      // 存储槽中的实现地址应该匹配当前逻辑合约地址
      // BigInt 转 address: 取后 20 字节
      const implFromSlot = "0x" + implAddress.slice(-40);
      expect(implFromSlot.toLowerCase()).to.equal(
        (await v2LogicNew.getAddress()).toLowerCase()
      );
    });
  });
});