// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

// ============================================
// Day 3 Part A: 控制流 (if/else / for / while)
// ============================================
contract ControlFlowDemo {
    // --- if/else ---
    // 记住: Solidity 没有 truthy/falsy，必须是显式 bool 表达式
    function getGrade(uint256 score) public pure returns (string memory) {
        if (score >= 90) {
            return "A";
        } else if (score >= 80) {
            return "B";
        } else if (score >= 70) {
            return "C";
        } else {
            return "D";
        }
    }

    // 三元运算符 (0.8+ 支持)
    function isEven(uint256 x) public pure returns (bool) {
        return x % 2 == 0 ? true : false;
    }

    // --- for 循环：计算数组和 ---
    function sumFor(uint256[] memory arr) public pure returns (uint256 total) {
        for (uint256 i = 0; i < arr.length; i++) {
            total += arr[i];
        }
        // ⚠️ arr.length 很大时 gas 爆炸 → 生产代码必须限制上限
    }

    // --- while 循环 ---
    function sumWhile(uint256[] memory arr) public pure returns (uint256 total) {
        uint256 i = 0;
        while (i < arr.length) {
            total += arr[i];
            i++;
            // 🚨 忘记 i++ = 无限循环 = gas 耗尽 = 交易回滚
        }
    }

    // --- break：提前跳出循环 ---
    function findFirstGreaterThan(
        uint256[] memory arr,
        uint256 threshold
    ) public pure returns (uint256 index, uint256 value) {
        for (uint256 i = 0; i < arr.length; i++) {
            if (arr[i] > threshold) {
                index = i;
                value = arr[i];
                break; // 找到即退出
            }
        }
        // 没找到 → index 和 value 保持默认值 0
    }

    // --- continue：跳过当前迭代 ---
    function sumEvenOnly(uint256[] memory arr) public pure returns (uint256 total) {
        for (uint256 i = 0; i < arr.length; i++) {
            if (arr[i] % 2 != 0) {
                continue; // 跳过奇数
            }
            total += arr[i];
        }
    }

    // --- do-while：先执行再判断 ---
    function factorial(uint256 n) public pure returns (uint256 result) {
        result = 1;
        if (n == 0) return result;

        do {
            result *= n;
            n--;
        } while (n > 0);
    }

    // --- 面试重点：如何防止 for 循环 gas 爆炸？---
    function safeBatchSum(uint256[] calldata arr) public pure returns (uint256 total) {
        require(arr.length <= 100, "Array too large"); // ✅ 限制输入规模
        for (uint256 i = 0; i < arr.length; i++) {
            total += arr[i];
        }
    }
}

// ============================================
// Day 3 Part B: 数组增删改查
// ============================================
contract ArrayDemo {
    uint256[] public numbers; // 动态数组（storage）

    // --- push：追加到末尾 ---
    function add(uint256 num) public {
        numbers.push(num); // 长度 +1
    }

    function getLength() public view returns (uint256) {
        return numbers.length;
    }

    function getAll() public view returns (uint256[] memory) {
        return numbers;
    }

    // --- pop：删除最后一个元素 ---
    function removeLast() public {
        require(numbers.length > 0, "Empty array");
        numbers.pop(); // 长度 -1，不返回值
    }

    // --- delete：重置为默认值（⚠️ 不是真删除！）---
    function resetAt(uint256 index) public {
        require(index < numbers.length, "Index out of bounds");
        delete numbers[index];
        // 位置 index 变成 0，但数组长度不变！
        // 例: [1, 2, 3] → delete [1] → [1, 0, 3]
    }

    // --- swap-and-pop：O(1) 删除任意位置（面试必考！）---
    // 不保持顺序，但效率最高
    function removeAt(uint256 index) public {
        require(index < numbers.length, "Index out of bounds");
        // 把最后一个元素拷到要删除的位置
        numbers[index] = numbers[numbers.length - 1];
        // 删除最后一个（现在是重复的）
        numbers.pop();
        // 例: [1, 2, 3, 4], removeAt(1) → [1, 4, 3]
    }

    // --- shrink：直接修改 length ---
    // function shrinkTo(uint256 newLength) public {
    //     require(newLength <= numbers.length, "Can only shrink");
    //     numbers.length = newLength; // ⚠️ Solidity 特有：直接改 length 截断
    // }

    // --- 定长数组 ---
    uint256[3] public fixedArr = [1, 2, 3];

    function updateFixed(uint256 index, uint256 value) public {
        require(index < 3, "Out of bounds");
        fixedArr[index] = value;
        // 定长数组没有 push() / pop()
    }
}

// ============================================
// Day 3 Part C: storage vs memory vs calldata 🔥
// ============================================
contract DataLocationDemo {
    uint256[] public storageArr;

    constructor() {
        storageArr.push(10);
        storageArr.push(20);
        storageArr.push(30);
    }

    // --- memory：在内存中创建临时数组 ---
    function buildMemoryArray(uint256 n) public pure returns (uint256[] memory) {
        uint256[] memory temp = new uint256[](n); // new 关键字分配内存
        for (uint256 i = 0; i < n; i++) {
            temp[i] = i * 10;
        }
        return temp; // 函数结束后 temp 自动销毁
    }

    // --- calldata：只读参数，最省 gas ---
    function sumCalldata(uint256[] calldata arr) public pure returns (uint256 total) {
        for (uint256 i = 0; i < arr.length; i++) {
            total += arr[i];
        }
        // arr[0] = 5; ← 编译错误！calldata 不可写
    }

    // --- memory 参数：可修改 ---
    function sumMemory(uint256[] memory arr) public pure returns (uint256 total) {
        arr[0] = 999; // ✅ memory 可以修改（但改了调用方的数组）
        for (uint256 i = 0; i < arr.length; i++) {
            total += arr[i];
        }
    }

    // --- storage 指针：修改指针 = 修改状态变量 ---
    function storagePointerDemo(uint256 value) public {
        uint256[] storage ptr = storageArr; // 指针！不是拷贝！
        ptr.push(value);                    // ✅ storageArr 也被修改了
    }

    // --- ⚠️ storage → memory 复制：大数组杀 gas ---
    function copyStorageToMemory() public view returns (uint256[] memory) {
        return storageArr; // 如果 storageArr 有 10000 个元素，直接耗尽 gas
    }

    // ================================
    // struct + 数据位置（面试高频！）
    // ================================
    struct User {
        string name;
        uint256 balance;
        bool active;
    }

    User[] public users;

    function addUser(string memory name, uint256 balance) public {
        users.push(User(name, balance, true));
    }

    // storage 引用 → 直接修改链上数据
    function updateUserBalance(uint256 index, uint256 newBalance) public {
        require(index < users.length, "Index out of bounds");
        User storage u = users[index]; // 指针
        u.balance = newBalance;        // ✅ 直接修改链上 users[index]
    }

    // memory 副本 → 不影响链上
    function readUserCopy(uint256 index) public view returns (User memory) {
        require(index < users.length, "Index out of bounds");
        User memory u = users[index]; // 完整复制到内存
        u.balance = 99999;            // 只改了内存副本
        return u;                     // 链上数据不变！
    }
}

// 数据位置赋值规则速查:
// ================================
// storage → storage  | 指针（引用）
// storage → memory   | 复制（⚠️ 开销大）
// memory  → memory   | 指针（引用）
// memory  → storage  | 复制
// calldata → memory  | 复制
// ================================
