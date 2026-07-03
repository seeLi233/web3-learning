// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

/**
 * OrderSystem.sol
 * 一个带完整事件体系的电商订单合约
 *
 * 练习目标：
 * 1. 设计合理的事件体系
 * 2. 理解哪些参数应该 indexed
 * 3. 使用自定义 error 替代 revert string
 * 4. 所有状态变更都有事件通知
 */

contract OrderSystem {
    // ============ 自定义 Error ============
    error OrderNotExist(uint256 orderId);
    error NotOrderOwner(uint256 orderId, address caller);
    error InvalidStatus(uint256 orderId, uint8 currentStatus, uint8 expectedStatus);
    error EmptyProductName();
    error ZeroAmount();

    // ============ 枚举 + 结构体 ============
     enum OrderStatus {
        None,        // 0 - 不存在
        Created,     // 1 - 已创建
        Paid,        // 2 - 已支付
        Shipped,     // 3 - 已发货
        Delivered,   // 4 - 已签收
        Cancelled,   // 5 - 已取消
        Refunded     // 6 - 已退款
    }

    struct Order {
        uint256 id;
        address buyer;
        string product;
        uint256 amount;        // 以 wei 为单位
        OrderStatus status;
        uint256 createdAt;
        uint256 updatedAt;
    }

    // ============ 状态变量 ============
    uint256 public nextOrderId = 1;
    mapping(uint256 => Order) public orders;

    // 记录每个用户的订单数量
    mapping(address => uint256) public userOrderCount;

    // 记录已取消的订单 ID（方便前端查询）
    uint256[] public cancelledOrderIds;

    // ============ 事件体系 ============
    // 设计原则：
    // 1. 所有状态变更都要发事件
    // 2. indexed 选择：用户前端需要按什么字段检索？
    // 3. 事件命名：统一用"主语 + 动词过去式"的格式

    // 新订单创建
    event OrderCreated(
        uint256 indexed orderId,    // 按 ID 检索单条
        address indexed buyer,      // "我的订单"页面
        string product,             // 不需要检索，直接展示
        uint256 amount,             // 不需要检索
        uint256 timestamp
    );

    // 订单状态变更（通用事件，所有状态变更复用）
    event OrderStatusChanged(
        uint256 indexed orderId,
        address indexed buyer,          // 用户的订单状态变更通知
        OrderStatus indexed oldStatus,  // 可以按旧状态过滤（如查所有"刚支付的"）
        OrderStatus newStatus,
        uint256 timestamp
    );

    // 退款事件（独立的，因为涉及金额）
    event OrderRefunded(
        uint256 indexed orderId,
        address indexed buyer,
        uint256 amount,
        uint256 timestamp
    );

    // 汇总统计事件（emit 给后端做数据分析）
    event DailyStats(
        uint256 indexed date,       // 按天索引
        uint256 totalOrders,        // data 区
        uint256 totalRevenue,       // data 区
        uint256 cancelledCount      // data 区
    );

    // ============ 核心函数 ============

    function createOrder(string calldata product)  external payable returns (uint256 orderId) {
        if (bytes(product).length == 0) revert EmptyProductName();
        if (msg.value == 0) revert ZeroAmount();

        orderId = nextOrderId++;

        orders[orderId] = Order({
            id: orderId,
            buyer: msg.sender,
            product: product,
            amount: msg.value,
            status: OrderStatus.Created,
            createdAt: block.timestamp,
            updatedAt: block.timestamp
        });

        userOrderCount[msg.sender]++;

        emit OrderCreated(orderId, msg.sender, product, msg.value, block.timestamp);
        emit OrderStatusChanged(
            orderId,
            msg.sender,
            OrderStatus.None,
            OrderStatus.Created,
            block.timestamp
        );
    }

    /// 支付订单（模拟，实际上已经在 createOrder 中支付了）
    function payOrder(uint256 orderId) external {
        Order storage order = orders[orderId];
        if (order.id == 0) revert OrderNotExist(orderId);
        if (order.status != OrderStatus.Created)
            revert InvalidStatus(orderId, uint8(order.status), uint8(OrderStatus.Created));

        order.status = OrderStatus.Paid;
        order.updatedAt = block.timestamp;

        emit OrderStatusChanged(
            orderId,
            order.buyer,
            OrderStatus.Created,
            OrderStatus.Paid,
            block.timestamp
        );
    }

    /// 发货（只有合约部署者可以调用，模拟商家操作）
    function shipOrder(uint256 orderId) external {
        Order storage order = orders[orderId];
        if (order.id == 0) revert OrderNotExist(orderId);
        if (order.status != OrderStatus.Paid)
            revert InvalidStatus(orderId, uint8(order.status), uint8(OrderStatus.Paid));

        order.status = OrderStatus.Shipped;
        order.updatedAt = block.timestamp;

        emit OrderStatusChanged(
            orderId,
            order.buyer,
            OrderStatus.Paid,
            OrderStatus.Shipped,
            block.timestamp
        );
    }

    /// 签收
    function confirmDelivery(uint256 orderId) external {
        Order storage order = orders[orderId];
        if (order.id == 0) revert OrderNotExist(orderId);
        if (order.buyer != msg.sender)
            revert NotOrderOwner(orderId, msg.sender);
        if (order.status != OrderStatus.Shipped)
            revert InvalidStatus(orderId, uint8(order.status), uint8(OrderStatus.Shipped));

        order.status = OrderStatus.Delivered;
        order.updatedAt = block.timestamp;

        emit OrderStatusChanged(
            orderId,
            order.buyer,
            OrderStatus.Shipped,
            OrderStatus.Delivered,
            block.timestamp
        );
    }

    /// 退款（合约部署者操作）
    function refundOrder(uint256 orderId) external {
        Order storage order = orders[orderId];
        if (order.id == 0) revert OrderNotExist(orderId);
        if (order.status == OrderStatus.Refunded)
            revert InvalidStatus(orderId, uint8(order.status), 255); // 255 表示"不允许此操作"

        // 退款给买家
        uint256 refundAmount = order.amount;
        order.status = OrderStatus.Refunded;
        order.updatedAt = block.timestamp;

        cancelledOrderIds.push(orderId);

        // ⚠️ 注意 CEI 模式：先改状态，再转账
        (bool success, ) = payable(order.buyer).call{value: refundAmount}("");
        require(success, "Refund transfer failed");

        emit OrderRefunded(orderId, order.buyer, refundAmount, block.timestamp);
        emit OrderStatusChanged(
            orderId,
            order.buyer,
            OrderStatus.Cancelled, // 假设从 Cancelled 退款
            OrderStatus.Refunded,
            block.timestamp
        );
    }

    /// 获取订单详情
    function getOrder(uint256 orderId) external view returns (Order memory) {
        if (orders[orderId].id == 0) revert OrderNotExist(orderId);
        return orders[orderId];
    }

    /// 获取用户所有订单（分页）
    function getUserOrderIds(
        address user,
        uint256 offset,
        uint256 limit
    ) external view returns (uint256[] memory) {
        // 注意：这是简化版，实际应该用可迭代 mapping 模式
        // 这里只做演示，返回空数组或简单实现
        uint256 total = userOrderCount[user];
        if (offset >= total) return new uint256[](0);

        uint256 end = offset + limit;
        if (end > total) end = total;
        uint256 resultSize = end - offset;

        uint256[] memory result = new uint256[](resultSize);

        // 遍历所有订单（⚠️ 生产环境不要这样，太费 gas）
        // 这是为了演示，实际上需要维护用户→订单列表的 mapping
        uint256 found = 0;
        for (uint256 i = 1; i < nextOrderId && found < resultSize; i++) {
            if (orders[i].buyer == user) {
                if (found >= offset) {
                    result[found - offset] = i;
                }
                found++;
                if (found >= end) break;
            }
        }

        return result;
    }

    // ============ 管理函数（演示用）============

    /// 发送每日统计事件（演示 emit 复杂 data）
    function emitDailyStats(uint256 date) external {
        // 实际项目中这会在每天 UTC 0:00 由 keeper bot 调用
        uint256 orderCount = nextOrderId - 1;
        uint256 cancelledCount = cancelledOrderIds.length;

        // 计算总收入（简化：遍历所有订单）
        uint256 totalRevenue = 0;
        for (uint256 i = 1; i < nextOrderId; i++) {
            if (orders[i].status != OrderStatus.Refunded) {
                totalRevenue += orders[i].amount;
            }
        }

        emit DailyStats(date, orderCount, totalRevenue, cancelledCount);
    }
}