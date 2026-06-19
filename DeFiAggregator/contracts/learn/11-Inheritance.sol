// SPDX-License-Identifier: SEE LICENSE IN LICENSE
pragma solidity ^0.8.0;

// =============================================
// 11-Inheritance.sol — 继承 + 菱形 + C3 线性化演示
// =============================================

//  ----- 1. 单继承基础 -----
contract Base {
    uint256 public value;

    event ValueChanged(uint256 oldValue, uint256 newValue);

    function setValue(uint256 _value) public virtual {
        uint256 old = value;
        value = _value;
        emit ValueChanged(old, _value);
    }

    function getValue() public view virtual returns (uint256) {
        return value;
    }
}

contract Child is Base {
    // 重写 setValue： 每次设置 +10
    function setValue(uint256 _value) public override {
        super.setValue(_value + 10);
    }

    // 重写 getValue：返回两倍
    function getValue() public view override returns (uint256) {
        return super.getValue() * 2;
    }
}

// ----- 2. 菱形继承 + C3 线性化演示 -----
contract A {
    event Log(string message);

    function foo() public virtual {
        emit Log("A.foo called");
    }
}

contract B is A {
    function foo() public virtual override {
        emit Log("B.foo called");
        super.foo();  // ← super 按 C3 顺序，不是直接调 A！
    }
}

contract C is A {
    function foo() public virtual override {
        emit Log("C.foo called");
        super.foo();
    }
}

contract D is B, C {
    // 声明: D is B, C
    // C3 线性化: D → B → C → A
    function foo() public override(B, C) {
        emit Log("D.foo called");
        super.foo();
    }
}
// 预期调用链: D → B → C → A
// 输出: "D.foo called" → "B.foo called" → "C.foo called" → "A.foo called"

// ----- 3. 声明顺序影响 C3 线性化 -----
contract E is C, B {
    // 声明: E is C, B
    // C3 线性化: E → C → B → A  ← 注意！B 和 C 的顺序变了
    function foo() public override(C, B) {
        emit Log("E.foo called");
        super.foo();
    }
}
// 预期调用链: E → C → B → A
// 输出: "E.foo called" → "C.foo called" → "B.foo called" → "A.foo called"

// ----- 4. 三层继承 + super 链 -----
contract Level1 {
    event Log(string message);

    function process(uint256 x) public virtual returns (uint256) {
        emit Log("Level1.process");
        return x;
    }
}

contract Level2 is Level1 {
    function process(uint256 x) public virtual override returns (uint256) {
        emit Log("Level2.process");
        return super.process(x + 1);
    }
}

contract Level3 is Level2 {
    function process(uint256 x) public override returns (uint256) {
        emit Log("Level3.process");
        return super.process(x + 1);
    }
}
// 调用 Level3.process(1):
//   Level3: x=1, super -> Level2
//   Level2: x=2, super -> Level1
//   Level1: x=3, return 3
//   最终返回: 3
//   事件: "Level3.process" → "Level2.process" → "Level1.process"