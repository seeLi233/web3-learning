import { expect } from "chai";
import { network } from "hardhat";

const { ethers } = await network.create();

describe("DeFiToken", function() {
    let token: any;
    let owner: any;
    let alice: any;
    let bob: any;

    const TOKEN_NAME = "DeFi Aggregator Token";
    const TOKEN_SYMBOL = "DEFI";
    const INITIAL_SUPPLY = ethers.parseEther("1000000"); // 100 万

    beforeEach(async function () {
        [owner, alice, bob] = await ethers.getSigners()
    
        // 使用 Hardhat 3 的方式部署
        token = await ethers.deployContract("DeFiToken", [TOKEN_NAME, TOKEN_SYMBOL, INITIAL_SUPPLY])
    });

    // ========== 部署测试 ==========
    describe("Deployment", function() {
        it("应该正确设置代币名称", async function () {
            expect(await token.name()).to.equal(TOKEN_NAME);
        });

        it("应该正确设置代币符号", async function () {
            expect(await token.symbol()).to.equal(TOKEN_SYMBOL);
        });

        it("应该正确设置小数位（默认18）", async function () {
            expect(await token.decimals()).to.equal(18n);
        });

        it("应该将初始供应量铸造给部署者", async function () {
            const ownerBalance = await token.balanceOf(owner.address);
            expect(ownerBalance).to.equal(INITIAL_SUPPLY);
        });

        it("总供应量应该等于初始供应量", async function () {
            expect(await token.totalSupply()).to.equal(INITIAL_SUPPLY);
        });

        it("部署者应该是 owner", async function () {
            expect(await token.owner()).to.equal(owner.address);
        });
    });

    // ========== 转账测试 ==========
    describe("Transfer", function() {
        it("应该能正常转账", async function () {
            const amount = ethers.parseEther("100");
            await token.transfer(alice.address, amount);

            expect(await token.balanceOf(alice.address)).to.equal(amount);
            expect(await token.balanceOf(owner.address)).to.equal(INITIAL_SUPPLY - amount);
        });

        it("余额不足时应该 revert", async function () {
            const tooMuch = INITIAL_SUPPLY + 1n;
            await expect(
                token.connect(alice).transfer(bob.address, tooMuch)
            ).to.be.revertedWithCustomError(token, "ERC20InsufficientBalance");
        });

        it("转账到零地址应该 revert", async function () {
            await expect(
                token.transfer(ethers.ZeroAddress, 100)
            ).to.be.revertedWithCustomError(token, "ERC20InvalidReceiver");
        });

        it("应该 emit Transfer 事件", async function () {
            const amount = ethers.parseEther("50");
            await expect(token.transfer(alice.address, amount))
                .to.emit(token, "Transfer")
                .withArgs(owner.address, alice.address, amount);
        });
    });

    // ========== 授权 + transferFrom 测试 ==========
    describe("Approve and TransferFrom", function() {
        it("应该能授权并 transferFrom", async function () {
            const amount = ethers.parseEther("200");
            await token.approve(alice.address, amount);

            expect(await token.allowance(owner.address, alice.address)).to.equal(amount);

            await token.connect(alice).transferFrom(owner.address, bob.address, amount);

            expect(await token.balanceOf(bob.address)).to.equal(amount);
        });

        it("授权额度不足时应该 revert", async function () {
            await token.approve(alice.address, 100);
            await expect(
                token.connect(alice).transferFrom(owner.address, bob.address, 200)
            ).to.be.revertedWithCustomError(token, "ERC20InsufficientAllowance");
        });

        it("应该 emit Approval 事件", async function () {
            const amount = ethers.parseEther("100");
            await expect(token.approve(alice.address, amount))
                .to.emit(token, "Approval")
                .withArgs(owner.address, alice.address, amount);
        });
    });

    // ========== 铸造测试 ==========
    describe("Mint", function() {
        it("owner 应该能铸造新代币", async function () {
            const amount = ethers.parseEther("50000");
            await token.mint(alice.address, amount);

            expect(await token.balanceOf(alice.address)).to.equal(amount);
            expect(await token.totalSupply()).to.equal(INITIAL_SUPPLY + amount);
        });

        it("非 owner 铸造应该 revert", async function () {
            await expect(token.connect(alice).mint(bob.address, 100)).to.be.revertedWithCustomError(token, "OwnableUnauthorizedAccount");
        });

        it("铸造到零地址应该 revert", async function () {
            await expect(
                token.mint(ethers.ZeroAddress, 100)
            ).to.be.revertedWithCustomError(token, "DeFiToken__MintToZeroAddress");
        });

        it("铸造零数量应该 revert", async function () {
            await expect(
                token.mint(alice.address, 0)
            ).to.be.revertedWithCustomError(token, "DeFiToken__MintZeroAmount");
        });

        it("铸造应该 emit TokensMinted 事件", async function () {
            const amount = ethers.parseEther("1000");
            await expect(token.mint(alice.address, amount))
                .to.emit(token, "TokensMinted")
                .withArgs(alice.address, amount);
        });
    });

    // ========== 燃烧测试 ==========
    describe("Burn", function() {
        beforeEach(async function () {
            // 先给 alice 转一些代币用于燃烧
            await token.transfer(alice.address, ethers.parseEther("1000"));
        });
        
        it("应该能燃烧自己的代币", async function () {
            const burnAmount = ethers.parseEther("300");
            await token.connect(alice).burn(burnAmount);

            expect(await token.balanceOf(alice.address)).to.be.equal(ethers.parseEther("700"));
            expect(await token.totalSupply()).to.equal(INITIAL_SUPPLY - burnAmount);
        });

        it("燃烧超过余额应该 revert", async function () {
            await expect(
                token.connect(alice).burn(ethers.parseEther("2000"))
            ).to.be.revertedWithCustomError(token, "ERC20InsufficientBalance");
        });

        it("应该 emit TokensBurned 事件", async function () {
            const burnAmount = ethers.parseEther("100");
            await expect(token.connect(alice).burn(burnAmount))
                .to.emit(token, "TokensBurned")
                .withArgs(alice.address, burnAmount);
        });

        it("应该支持 burnFrom — 授权后燃烧他人代币", async function () {
            const approveAmount = ethers.parseEther("500");
            const burnAmount = ethers.parseEther("200");

            // alice 授权 bob 燃烧自己的代币
            await token.connect(alice).approve(bob.address, approveAmount);

            // bob 燃烧 alice 的代币
            await token.connect(bob).burnFrom(alice.address, burnAmount);

            // 验证余额
            expect(await token.balanceOf(alice.address)).to.equal(
                ethers.parseEther("1000") - burnAmount
            );
            // 验证 allowance 减少了
            expect(await token.allowance(alice.address, bob.address)).to.equal(
                approveAmount - burnAmount
            );
            // 验证总供应量减少
            expect(await token.totalSupply()).to.equal(INITIAL_SUPPLY - burnAmount);
        });

        it("burnFrom 超出授权额度应该 revert", async function () {
            await token.connect(alice).approve(bob.address, ethers.parseEther("100"));

            await expect(
                token.connect(bob).burnFrom(alice.address, ethers.parseEther("200"))
            ).to.be.revertedWithCustomError(token, "ERC20InsufficientAllowance");
        });

        it("burnFrom 应该 emit TokensBurned 事件", async function () {
            const burnAmount = ethers.parseEther("100");
            await token.connect(alice).approve(bob.address, burnAmount);

            await expect(token.connect(bob).burnFrom(alice.address, burnAmount))
                .to.emit(token, "TokensBurned")
                .withArgs(alice.address, burnAmount);
        });
    });

    describe("batchTransfer — 批量转账", function () {
        it("应该支持批量转账", async function () {
            const amounts = [ethers.parseEther("100"), ethers.parseEther("200")];
            await token.batchTransfer([alice.address, bob.address], amounts);

            expect(await token.balanceOf(alice.address)).to.equal(amounts[0]);
            expect(await token.balanceOf(bob.address)).to.equal(amounts[1]);
        });

        it("数组长度不一致应该 revert", async function () {
            await expect(
                token.batchTransfer([alice.address], [100n, 200n])
            ).to.be.revertedWithCustomError(token, "DeFiToken__BatchLengthMismatch");  // 改自定义 error 后可以用 revertedWithCustomError
        });

        it("空数组应该 revert", async function () {
            await expect(
                token.batchTransfer([], [])
            ).to.be.revertedWithCustomError(token, "DeFiToken__EmptyBatch");
        });
    });

    // ========== 集成测试：完整流程 ==========
    describe("Integration", function() {
        it("完整的 mint -> transfer -> approve -> transferFrom -> burn 流程", async function () {
            // 1. Owner 铸造 1000 token 给 Alice
            const mintAmount = ethers.parseEther("1000");
            await token.mint(alice.address, mintAmount);
            expect(await token.balanceOf(alice.address)).equal(mintAmount);

            // 2. Alice 转 300 token 给 Bob
            const transferAmount = ethers.parseEther("300");
            await token.connect(alice).transfer(bob.address, transferAmount);
            expect(await token.balanceOf(alice.address)).to.be.equal(ethers.parseEther("700"));
            expect(await token.balanceOf(bob.address)).to.be.equal(transferAmount);
        
            // 3. Bob 授权 Alice 100 token
            await token.connect(bob).approve(alice.address, ethers.parseEther("100"));

            // 4. Alice 从 Bob 账户转 100 token 给自己
            await token.connect(alice).transferFrom(bob.address, alice.address, ethers.parseEther("100"));
            expect(await token.balanceOf(alice.address)).to.be.equal(ethers.parseEther("800"));

            // 5. Bob 燃烧自己剩余的 200 token
            await token.connect(bob).burn(ethers.parseEther("200"));
            expect(await token.balanceOf(bob.address)).to.be.equal(0);

            // 6. 验证最终总供应量
            // 初始 100 万 + 铸造 1000 - 燃烧 200 = 100 万 + 800
            const expectedTotal = INITIAL_SUPPLY + ethers.parseEther("1000") - ethers.parseEther("200");
            expect(await token.totalSupply()).to.equal(expectedTotal);
        });

        it("完整治理流程: permit签名授权 → transferFrom → 委托聚合 → burn → 投票权重联动", async function () {
            const permitAmount = ethers.parseEther("5000");
            const burnAmount = ethers.parseEther("1000");

            // ============================================================
            // Step 1: owner 离线签名 permit，授权 bob 使用 5000 token
            //         （这就是 gasless approve — 用户不用付 gas 做授权！）
            // ============================================================
            const nonce = await token.nonces(owner.address);
            const deadline = Math.floor(Date.now() / 1000) + 3600;
            const chainId = (await ethers.provider.getNetwork()).chainId;

             const domain = {
                name: TOKEN_NAME,
                version: "1",
                chainId: Number(chainId),
                verifyingContract: await token.getAddress(),
            };

            const types = {
                Permit: [
                    { name: "owner", type: "address" },
                    { name: "spender", type: "address" },
                    { name: "value", type: "uint256" },
                    { name: "nonce", type: "uint256" },
                    { name: "deadline", type: "uint256" },
                ],
            };

            const message = {
                owner: owner.address,
                spender: bob.address,
                value: permitAmount,
                nonce: nonce,
                deadline: deadline,
            };

            const signature = await owner.signTypedData(domain, types, message);
            const sig = ethers.Signature.from(signature);

            // bob 拿着签名调用 permit — 一笔交易完成授权（owner 不用付 gas！）
            await token.connect(bob).permit(
                owner.address, bob.address, permitAmount, deadline,
                sig.v, sig.r, sig.s
            );
            expect(await token.allowance(owner.address, bob.address)).to.equal(permitAmount);

            // ============================================================
            // Step 2: bob 用刚获得的授权额度，从 owner 账户转走 5000 token
            // ============================================================
            await token.connect(bob).transferFrom(owner.address, bob.address, permitAmount);
            expect(await token.balanceOf(bob.address)).to.equal(permitAmount);

            // owner 剩余 = INITIAL_SUPPLY - 5000
            const ownerRemaining = INITIAL_SUPPLY - permitAmount;
            expect(await token.balanceOf(owner.address)).to.equal(ownerRemaining);

            // ============================================================
            // Step 3: 治理权聚合 — owner 和 bob 都把投票权委托给 alice
            //         面试亮点: 多人委托同一人 = 投票权聚合，DAO 的核心机制
            // ============================================================
            // owner 委托给 alice
            await token.delegate(alice.address);
            expect(await token.getVotes(alice.address)).to.equal(ownerRemaining);

            // bob 也委托给 alice → 投票权叠加！
            await token.connect(bob).delegate(alice.address);
            // alice = owner余额 + bob余额 = (INITIAL_SUPPLY - 5000) + 5000 = INITIAL_SUPPLY
            expect(await token.getVotes(alice.address)).to.equal(INITIAL_SUPPLY);

            // ============================================================
            // Step 4: bob burn 掉 1000 token → alice 的投票权重自动联动减少
            //         面试亮点: burn 不只是通缩，还会影响治理投票权！
            // ============================================================
            const votesBeforeBurn = await token.getVotes(alice.address);
            // 保存 burn 前的区块号，用于 Step 5 的快照验证
            const blockBeforeBurn = await ethers.provider.getBlockNumber();

            const blockData = await ethers.provider.getBlock(blockBeforeBurn);
            const timestampBeforeBurn = blockData!.timestamp;

            await token.connect(bob).burn(burnAmount);

            // alice 聚合的投票权重应减少 1000（因为 bob 余额减少，bob 委托给了 alice）
            expect(await token.getVotes(alice.address)).to.equal(votesBeforeBurn - burnAmount);
            // bob 余额验证
            expect(await token.balanceOf(bob.address)).to.equal(permitAmount - burnAmount);

            // ============================================================
            // Step 5: 快照验证 — burn 前的历史投票权重不受影响
            //         面试亮点: getPastVotes 读取的是历史快照，不是当前值
            //                   这就是防"投票后转账再投票"的核心机制！
            // ============================================================
            const pastVotes = await token.getPastVotes(alice.address, timestampBeforeBurn);
            // blockBeforeBurn 那个区块，burn 还没发生，alice 的权重是 burn 前的值
            expect(pastVotes).to.equal(votesBeforeBurn);

            // ============================================================
            // Step 6: 最终状态校验
            // ============================================================
            // 总供应量 = INITIAL_SUPPLY - burnAmount
            expect(await token.totalSupply()).to.equal(INITIAL_SUPPLY - burnAmount);
            // bob 已经委托给 alice，bob 自己的 votes = 0
            expect(await token.delegates(bob.address)).to.equal(alice.address);
            // alice 的聚合权重 = owner余额 + bob余额(after burn)
            // = (INITIAL_SUPPLY - 5000) + (5000 - 1000) = INITIAL_SUPPLY - 1000
            expect(await token.getVotes(alice.address)).to.equal(INITIAL_SUPPLY - burnAmount);
        });
    });

    // ==================== Part 2: ERC20Permit (EIP-2612) ====================
    describe("ERC20Permit — Gasless Approve", function() {
        it("应该支持 permit 签名授权", async function () {
            const amount = 1_000_000_000_000_000_000n;
            const nonce = await token.nonces(owner.address);
            const deadline = Math.floor(Date.now() / 1000) + 3600; // 1 小时后过期
            const chainId = (await ethers.provider.getNetwork()).chainId;
            const tokenAddress = await token.getAddress();

            // 构造 EIP-712 域名
            const domain = {
                name: TOKEN_NAME,
                version: "1",
                chainId: Number(chainId),
                verifyingContract: tokenAddress,
            };

            // 构造 Permit 类型
            const types = {
                Permit: [
                    {name:"owner", type:"address"},
                    {name:"spender", type:"address"},
                    {name:"value", type:"uint256"},
                    {name:"nonce", type:"uint256"},
                    {name:"deadline", type:"uint256"},
                ],
            };

            // 构造签名消息
            const message = {
                owner: owner.address,
                spender: bob.address,
                value: amount,
                nonce: nonce,
                deadline: deadline
            };

            // owner 离线签名 (实际项目中用户在 MetaMask 中签名)
            const signature = await owner.signTypedData(domain, types, message);
            const sig = ethers.Signature.from(signature);

            // Bob 拿着签名调用 permit，帮 owner 完成授权
            // 注意：Bob 调用 permit，但授权的是 owner → bob
            await token.connect(bob).permit(
                owner.address,
                bob.address,
                amount,
                deadline,
                sig.v,
                sig.r,
                sig.s
            );

            // 验证授权成功
            expect(await token.allowance(owner.address, bob.address)).to.equal(amount);
        });

        it("过期的 deadline 应该 revert", async function () {
            const nonce = await token.nonces(owner.address);
            const pastDeadline = Math.floor(Date.now() / 1000) - 3600; // 1 小时前
            const chainId = (await ethers.provider.getNetwork()).chainId;

            const domain = {
                name: TOKEN_NAME,
                version: "1",
                chainId: Number(chainId),
                verifyingContract: await token.getAddress(),
            };

            const types = {
                Permit: [
                    {name:"owner", type:"address"},
                    {name:"spender", type:"address"},
                    {name:"value", type:"uint256"},
                    {name:"nonce", type:"uint256"},
                    {name:"deadline", type:"uint256"},
                ],
            };

            const message = {
                owner: owner.address,
                spender: bob.address,
                value: 100n,
                nonce: nonce,
                deadline: pastDeadline, // <- 关键：过去的时间
            };

            const signature = await owner.signTypedData(domain, types, message);
            const sig = ethers.Signature.from(signature);

            // 应该因为过期而 revert
            await expect(
                token.connect(bob).permit(
                    owner.address, bob.address, 100n, pastDeadline,
                    sig.v, sig.r, sig.s
                )
            ).to.be.revertedWithCustomError(token, "ERC2612ExpiredSignature");
        });

        it("重复使用相同签名应该 revert（nonce 防重放）", async function () {
            const nonce = await token.nonces(owner.address);
            const deadline = Math.floor(Date.now() / 1000) + 3600;
            const chainId = (await ethers.provider.getNetwork()).chainId;

            const domain = {
                name: TOKEN_NAME,
                version: "1",
                chainId: Number(chainId),
                verifyingContract: await token.getAddress(),
            };

            const types = {
                Permit: [
                    {name:"owner", type:"address"},
                    {name:"spender", type:"address"},
                    {name:"value", type:"uint256"},
                    {name:"nonce", type:"uint256"},
                    {name:"deadline", type:"uint256"},
                ],
            };

            const message = {
                owner: owner.address,
                spender: bob.address,
                value: 100n,
                nonce: nonce,
                deadline: deadline,
            };

            const signature = await owner.signTypedData(domain, types, message);
            const sig = ethers.Signature.from(signature);

            // 第一次 permit 成功
            await token.connect(bob).permit(
                owner.address, bob.address, 100n, deadline,
                sig.v, sig.r, sig.s
            );

            // 第二次使用相同签名 — nonce 已经变了，应该 revert
            await expect(
                token.connect(bob).permit(
                    owner.address, bob.address, 100n, deadline,
                    sig.v, sig.r, sig.s
                )
            ).to.be.revertedWithCustomError(token, "ERC2612InvalidSigner");
        });
    });

    // ==================== Part 4: ERC20Votes ====================
    describe("ERC20Votes - 治理投票", function() {
        it("应该支持委托投票 (delegate)", async function () {
            // owner 把投票权委托给 alice
            await token.delegate(alice.address);
            
            // alice 现在拥有 owner 的投票权重
            const votes = await token.getVotes(alice.address);
            expect(votes).to.equal(INITIAL_SUPPLY);
        });

        it("委托后转账，投票权重应该跟着转移", async function () {
            const amount = ethers.parseEther("300");
            // owner 委托给 alice
            await token.delegate(alice.address);

            // owner 转 300 给 bob
            await token.transfer(bob.address, amount);

            // alice 的投票权重应该减少 300
            const aliceVotes = await token.getVotes(alice.address);
            expect(aliceVotes).to.equal(INITIAL_SUPPLY - amount);
        });

        it("getPastVotes 应该返回历史快照值", async function () {
            // owner 先委托
            await token.delegate(alice.address);
            const blockAfterDelegate = await ethers.provider.getBlockNumber();

            const blockAfterDelegateData = await ethers.provider.getBlock(blockAfterDelegate);
            const timestampAfterDelegate = blockAfterDelegateData!.timestamp;

            // 然后转账
            await token.transfer(alice.address, ethers.parseEther("500"));

            // 查询"委托后、转账前"那个区块的投票权重
            // 应该仍然是 INITIAL_SUPPLY，因为快照机制！
            const pastVotes = await token.getPastVotes(
                alice.address,
                timestampAfterDelegate
            );
            expect(pastVotes).to.equal(INITIAL_SUPPLY);
        });

        it("应该正确更新 delegates 映射", async function () {
            await token.delegate(alice.address);

            // delegates(owner) 应该返回 alice
            expect(await token.delegates(owner.address)).to.equal(alice.address);
        });

        it("clock 应该返回当前时间戳", async function () {
            const clock = await token.clock();
            const now = BigInt(Math.floor(Date.now() / 1000));
            // clock 应该接近当前时间（允许 5 秒误差）
            expect(clock).to.be.closeTo(now, 120n);
        });

        it("CLOCK_MODE 应该返回 'mode=timestamp'", async function () {
            expect(await token.CLOCK_MODE()).to.equal("mode=timestamp");
        });
    });
});
