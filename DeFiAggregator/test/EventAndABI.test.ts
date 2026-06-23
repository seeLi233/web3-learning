import { expect } from "chai";
import { network } from "hardhat";

const { ethers } = await network.create();

describe("Day 7: 事件深入 + ABI 编码解码", function () {
    // ============================================
    // Part 1: EventDeepDive 测试
    // ============================================
    describe("EventDeepDive", function() {
        let contract:any;

        beforeEach(async function () {
            contract = await ethers.deployContract("EventDeepDive");
        });

        it('1.1 BasicEvent — 验证 topics 和 data 结构', async function () {
            const tx = await contract.emitBasic(42, 'hello world');
            const receipt = await tx.wait();

            // receipt.logs[0] 就是事件日志
            const log = receipt.logs[0];

            // ----- topics -----
            // log.topics 是一个数组，长度不固定
            // 但我们知道 BasicEvent 有 2 个 indexed 参数 + 事件签名 = 3 个 topic
            console.log('=== 日志结构分析 ===');
            console.log('合约地址:', log.address);
            console.log('topics 数量:', log.topics.length);

            // topic[0] = 事件签名 hash
            const eventSignature = 'BasicEvent(address,uint256,string,uint256)';
            const expectedTopic0 = ethers.keccak256(
                ethers.toUtf8Bytes(eventSignature)
            );
            expect(log.topics[0]).to.equal(expectedTopic0);

            // topic[1] = msg.sender（第一个 indexed 参数）
            // 地址会被左补 0 到 32 字节，所以用 hexZeroPad
            console.log('topic[1] (sender):', log.topics[1]);

            // topic[2] = id（第二个 indexed 参数，uint256）
            console.log('topic[2] (id):', log.topics[2]);

            // ----- data -----
            // data 里包含非 indexed 参数: string message, uint256 timestamp
            console.log('data:', log.data);

            // 用 ABI 解码 data 部分
            // BasicEvent 的非 indexed 参数: string, uint256
            const decoded = ethers.AbiCoder.defaultAbiCoder().decode(
                ['string', 'uint256'],
                log.data
            );
            console.log('decoded message:', decoded[0]);
            console.log('decoded timestamp:', decoded[1]);
            expect(decoded[0]).to.equal('hello world');
        });

        it('1.2 匿名事件 — 验证 topic[0] 不是事件签名', async function () {
            const tx = await contract.emitAnonymous(
                '0x1111111111111111111111111111111111111111',
                '0x2222222222222222222222222222222222222222',
                '0x3333333333333333333333333333333333333333',
                '0x4444444444444444444444444444444444444444'
            );
            const receipt = await tx.wait();

            // 匿名事件的 log：topics 中不包含事件签名
            // topcis 直接就是 4 个 indexed 参数
            const log = receipt.logs[0];
            console.log('匿名事件 topics 数量:', log.topics.length); // 4
            console.log('topic[0] (a):', log.topics[0]); // 第一个 indexed 参数
            console.log('topic[1] (b):', log.topics[1]); // 第二个 indexed 参数
            console.log('topic[2] (c):', log.topics[2]);
            console.log('topic[3] (d):', log.topics[3]);

            // 验证 topic[0] 不是事件签名
            const signature = 'AnonymousEvent(address,address,address,address)';
            const sigHash = ethers.keccak256(ethers.toUtf8Bytes(signature));
            expect(log.topics[0]).to.not.equal(sigHash);
        });

        it('1.3 Gas 对比 — LightEvent vs HeavyEvent', async function () {
            // 测试 LightEvent
            const txLight = await contract.emitLight(42);
            const receiptLight = await txLight.wait();
            console.log('LightEvent gas used:', receiptLight.gasUsed.toString());

            // 测试 HeavyEvent
            const txHeavy = await contract.emitHeavy(
                1,
                '0x1111111111111111111111111111111111111111',
                ethers.ZeroHash,
                'hello world',
                'longer string here',
                [1, 2, 3, 4, 5]
            );
            const receiptHeavy = await txHeavy.wait();
            console.log('HeavyEvent gas used:', receiptHeavy.gasUsed.toString());

            // HeavyEvent 应该消耗更多 gas
            expect(receiptHeavy.gasUsed).to.be.greaterThan(receiptLight.gasUsed);
        });

        it('1.4 getEventSignature — 手动计算事件签名', async function () {
            const sig = await contract.getEventSignature(
                'Transfer(address,address,uint256)'
            );
            console.log('Transfer 事件签名:', sig);

            // ERC20 Transfer 事件的签名是固定的：
            // 0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef
            const expected = ethers.keccak256(
                ethers.toUtf8Bytes('Transfer(address,address,uint256)')
            );
            expect(sig).to.equal(expected);
        });
    });

    // ============================================
    // Part 2: ABIEncoding 测试
    // ============================================
    describe('ABIEncoding', function () {
        let contract: any;

        beforeEach(async function () {
           contract = await ethers.deployContract("ABIEncoding"); 
        });

        it('2.1 abi.encode vs abi.encodePacked — 长度对比', async function () {
            const addr = '0x5B38Da6a701c568545dCfcB03FcB875f56beddC4';

            const encoded = await contract.demoEncode(42, addr);
            const packed = await contract.demoEncodePacked(42, addr);

            console.log('abi.encode 长度:', encoded.length, 'bytes');      // 64
            console.log('abi.encodePacked 长度:', packed.length, 'bytes');  // 36

            // encode 应该比 encodePacked 长（因为对齐补 0）
            expect(encoded.length).to.be.greaterThan(packed.length);
        });

        it('2.2 encodePacked 碰撞风险 — 面试必考！', async function () {
            const result = await contract.collisionDemo();

            console.log('hash1 (ab, c):', result.hash1);
            console.log('hash2 (a, bc):', result.hash2);
            console.log('areEqual:', result.areEqual);

            // hash1 == hash2！这就是碰撞
            expect(result.areEqual).to.be.true;
        });

        it('2.3 abi.encodeWithSelector vs encodeWithSignature', async function () {
            const to = '0x5B38Da6a701c568545dCfcB03FcB875f56beddC4';
            const amount = ethers.parseEther("1.0"); // 1 ether

            const withSelector = await contract.demoEncodeWithSelector(to, amount);
            const withSignature = await contract.demoEncodeWithSignature(to, amount);

            console.log('encodeWithSelector:', withSelector);
            console.log('encodeWithSignature:', withSignature);

            // 结果应该完全相同
            expect(withSelector).to.equal(withSignature);

            // 前 4 字节 = transfer 函数选择器
            const selector = ethers.id('transfer(address,uint256)').substring(0, 10);
            console.log('函数选择器（前 4 字节）:', selector);
            expect(withSelector.substring(0, 10)).to.equal(selector);
        });

        it('2.4 abi.decode — 编解码往返', async function () {
            const x = BigInt(123456);
            const addr = '0x5B38Da6a701c568545dCfcB03FcB875f56beddC4';

            const result = await contract.roundTrip(x, addr);

            console.log('原始 x:', x.toString());
            console.log('解码 x:', result.decodedX.toString());
            console.log('原始 addr:', addr);
            console.log('解码 addr:', result.decodedAddr);

            expect(result.decodedX).to.equal(x);
            expect(result.decodedAddr.toLowerCase()).to.equal(addr.toLowerCase());
        });

        it('2.5 函数选择器计算', async function () {
            const selector = await contract.getSelector(
                'transfer(address,uint256)'
            );
            console.log('transfer 选择器:', selector);

            // 验证与 ethers 计算结果一致
            const expectedSelector = ethers.id('transfer(address,uint256)').substring(0, 10);
            expect(selector).to.equal(expectedSelector);
        });
    });

    // ============================================
    // Part 3: OrderSystem 测试 — 复杂事件实践
    // ============================================
    describe('OrderSystem', function () {
        let contract: any;
        let buyer: any;

        beforeEach(async function () {
            contract = await ethers.deployContract("OrderSystem");
            buyer = await ethers.getSigners();
        });

        it('3.1 创建订单 — 验证 OrderCreated 事件', async function () {
            const tx = await contract.createOrder('iPhone 15 Pro', {
                value: ethers.parseEther('0.5'),
            });
            const receipt = await tx.wait();

            // 手动解析 logs，找到 OrderCreated 事件
            const eventSignature = ethers.keccak256(
                ethers.toUtf8Bytes(
                    'OrderCreated(uint256,address,string,uint256,uint256)'
                )
            );

            const orderCreatedLog = receipt.logs.find(
                (log: any) => log.topics[0] === eventSignature
            );

            expect(orderCreatedLog).to.not.be.undefined;

            // 解析 indexed 参数 (从 topics)
            // topic[1] = orderId (uint256)
            const orderId = BigInt(orderCreatedLog!.topics[1]);
            console.log('订单 ID:', orderId.toString());

            // topic[2] = buyer (address)
            const buyerAddress = '0x' + orderCreatedLog!.topics[2].slice(26);
            console.log('买家:', buyerAddress);

            // 解析 data 参数 (非 indexed)
            const decoded = ethers.AbiCoder.defaultAbiCoder().decode(
                ['string', 'uint256', 'uint256'],  // product, amount, timestamp
                orderCreatedLog!.data
            );

            console.log('商品:', decoded[0]);
            console.log('金额:', ethers.formatEther(decoded[1]), 'ETH');
            console.log('时间戳:', decoded[2].toString());

            expect(decoded[0]).to.equal('iPhone 15 Pro');
            expect(decoded[1]).to.equal(ethers.parseEther('0.5'));
        });

        it('3.2 完整订单流程 — 验证状态变更事件链', async function () {
            // 1. 创建
            const tx1 = await contract.createOrder('MacBook Pro', {
                value: ethers.parseEther('1.0'),
            });
            const r1 = await tx1.wait();

            const eventSig = ethers.keccak256(
                ethers.toUtf8Bytes(
                    'OrderStatusChanged(uint256,address,uint8,uint8,uint256)'
                )
            );
            const log1 = r1.logs.find((l: any) => l.topics[0] === eventSig);
            const orderId = BigInt(log1!.topics[1]);
            console.log('\n=== 订单 #' + orderId.toString() + ' 状态流转 ===');

            // 解析状态变更
            const decoded1 = ethers.AbiCoder.defaultAbiCoder().decode(
                ['uint8', 'uint256'],
                log1!.data
            );
            const oldStatus1 = BigInt(log1!.topics[3]);
            console.log('状态: None(', oldStatus1, ') → Created(', decoded1[0], ')');

            // 2. 支付
            const tx2 = await contract.payOrder(orderId);
            const r2 = await tx2.wait();
            const log2 = r2.logs.find((l: any) => l.topics[0] === eventSig);
            const decoded2 = ethers.AbiCoder.defaultAbiCoder().decode(
                ['uint8', 'uint256'],
                log2!.data
            );
            const oldStatus2 = BigInt(log2!.topics[3]);  // ← 从 topic[3] 读
            console.log('状态: Created(', oldStatus2, ') → Paid(', decoded2[0], ')');

            // 3. 发货
            const tx3 = await contract.shipOrder(orderId);
            const r3 = await tx3.wait();
            const log3 = r3.logs.find((l: any) => l.topics[0] === eventSig);
            const decoded3 = ethers.AbiCoder.defaultAbiCoder().decode(
                ['uint8', 'uint256'],
                log3!.data
            );
            const oldStatus3 = BigInt(log3!.topics[3]);
            console.log('状态: Paid(', oldStatus3, ') → Shipped(', decoded3[0], ')');

            // 4. 签收
            const tx4 = await contract.confirmDelivery(orderId);
            const r4 = await tx4.wait();
            const log4 = r4.logs.find((l: any) => l.topics[0] === eventSig);
            const decoded4 = ethers.AbiCoder.defaultAbiCoder().decode(
                ['uint8', 'uint256'],
                log4!.data
            );
            const oldStatus4 = BigInt(log4!.topics[3]);
            console.log('状态: Shipped(', oldStatus4, ') → Delivered(', decoded4[0], ')');

            expect(decoded4[0]).to.equal(4); // Delivered = 4
        });

        it('3.3 自定义 error 测试', async function () {
            // 测试 OrderNotExist
            await expect(
                contract.payOrder(99999)
            ).to.be.revertedWithCustomError(contract, 'OrderNotExist');
        });

        it('3.4 DailyStats — 复杂 data 事件', async function () {
            // 先创建几个订单
            await contract.createOrder('Item A', {
                value: ethers.parseEther('0.1'),
            });
            await contract.createOrder('Item B', {
                value: ethers.parseEther('0.2'),
            });

            const today = Math.floor(Date.now() / 1000 / 86400); // 今天的日期编号
            const tx = await contract.emitDailyStats(today);
            const receipt = await tx.wait();

            const eventSig = ethers.keccak256(
                ethers.toUtf8Bytes('DailyStats(uint256,uint256,uint256,uint256)')
            );
            const log = receipt.logs.find((l: any) => l.topics[0] === eventSig);

            expect(log).to.not.be.undefined;

            // topic[1] = date (indexed)
            const date = BigInt(log!.topics[1]);
            console.log('统计日期:', date.toString());

            // data: totalOrders, totalRevenue, cancelledCount
            const decoded = ethers.AbiCoder.defaultAbiCoder().decode(
                ['uint256', 'uint256', 'uint256'],
                log!.data
            );
            console.log('总订单数:', decoded[0].toString());
            console.log('总收入:', ethers.formatEther(decoded[1]), 'ETH');
            console.log('取消数:', decoded[2].toString());
        });
    });
});

