# [Stake需求文档](https://github.com/MetaNodeAcademy/Base2-Solidity-stake) 👈 
[Stake前端代码](https://github.com/MetaNodeAcademy/Base2-Solidity-stake/stake/tree/main/stake-fe) 👈 
[Stake合约代码](https://github.com/MetaNodeAcademy/Base2-Solidity-stake/stake/tree/main/stake-contract) 👈 

## 需求文档
建议：

1. 需要写测试！！！
2. 合约开发完成后部署到 sepolia 进行测试
3. 可以自己在sepolia 上面发一个erc20 token 作为reward token的代币
4. 第一个stake token 是所在链的native currency；如果想开其他pool，stake token 可以是erc20Token，合约自行改造调整。

## 1️⃣ 系统概述

Stake 是一个基于区块链的质押系统，支持多种代币的质押，并基于用户质押的代币数量和时间长度分配 MetaNode代币作为奖励。系统可提供多个质押池，每个池可以独立配置质押代币、奖励计算等。

## 2️⃣ 功能需求

### 2.1 质押功能

- **输入参数**: 池 ID(_pid)，质押数量(_amount)。
- **前置条件**: 用户已授权足够的代币给合约。
- **后置条件**: 用户的质押代币数量增加，池中的总质押代币数量更新。
- **异常处理**: 质押数量低于最小质押要求时拒绝交易。

### 2.2 解除质押功能

- **输入参数**: 池 ID(_pid)，解除质押数量(_amount)。
- **前置条件**: 用户质押的代币数量足够。
- **后置条件**: 用户的质押代币数量减少，解除质押请求记录，等待锁定期结束后可提取。
- **异常处理**: 如果解除质押数量大于用户质押的数量，交易失败。

### 2.3 领取奖励

- **输入参数**: 池 ID(_pid)。
- **前置条件**: 有可领取的奖励。
- **后置条件**: 用户领取其奖励，清除已领取的奖励记录。
- **异常处理**: 如果没有可领取的奖励，不执行任何操作。

### 2.4 添加和更新质押池

- **输入参数**: 质押代币地址(_stTokenAddress)，池权重(_poolWeight)，最小质押金额(_minDepositAmount)，解除质押锁定区(_unstakeLockedBlocks)。
- **前置条件**: 只有管理员可操作。
- **后置条件**: 创建新的质押池或更新现有池的配置。
- **异常处理**: 权限验证失败或输入数据验证失败。

### 2.5 合约升级和暂停

- **升级合约**: 只有持有升级角色的账户可执行。
- **暂停/恢复操作**: 可以独立控制质押、解除质押、领奖等操作的暂停和恢复。

## 3️⃣ 数据结构

### 3.1 Pool

- **stTokenAddress**: 质押代币的地址。
- **poolWeight**: 质押池的权重，影响奖励分配。
- **lastRewardBlock**: 最后一次计算奖励的区块号。
- **accMetaNodePerST**: 每个质押代币累积的 RCC 数量。
- **stTokenAmount**: 池中的总质押代币量。
- **minDepositAmount**: 最小质押金额。
- **unstakeLockedBlocks**: 解除质押的锁定区块数。

### 3.2 User

- **stAmount**: 用户质押的代币数量。
- **finishedMetaNode**: 已分配的 MetaNode数量。
- **pendingMetaNode**: 待领取的 MetaNode 数量。
- **requests**: 解质押请求列表，每个请求包含解质押数量和解锁区块。

![alt text](image.png)

MetaNodeToken 部署在: 0xF74071daf669ACEEE6A4eaC5e35E63a3EC3175ec
MetaNodeStake 部署在: 0x2e125fBF389Bc47c6919BC2Ef6CD312D0908f6DB