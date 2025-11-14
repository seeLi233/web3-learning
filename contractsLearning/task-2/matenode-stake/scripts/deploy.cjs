const { ethers } = require("hardhat");

async function main() {
    console.log("开始部署 MetaNode 质押系统...");

    // 部署奖励代币
    const MetaNodeToken = await ethers.getContractFactory("MetaNodeToken");
    const metaNodeToken = await MetaNodeToken.deploy((await ethers.getSigners())[0].address);
    await metaNodeToken.waitForDeployment();
    console.log(`MetaNodeToken 部署在: ${ await metaNodeToken.getAddress() }`);

    // 部署质押合约
    const rewardPerBlock = ethers.parseEther("10"); // 每个区块 10 个奖励币
    const MetaNodeStake = await ethers.getContractFactory("MetaNodeStake");
    const metaNodeStake = await MetaNodeStake.deploy(await metaNodeToken.getAddress(), rewardPerBlock, (await ethers.getSigners())[0].address);
    await metaNodeStake.waitForDeployment();
    console.log(`MetaNodeStake 部署在: ${ await metaNodeStake.getAddress() }`);

    // 授权质押合约铸造权限
    await metaNodeToken.grantRole(await metaNodeToken.MINTER_ROLE(), await metaNodeStake.getAddress());
    console.log("已授权质押合约铸造奖励代币")
}

main().then(() => process.exit(0)).catch((error) => {
    console.log(error);
    process.exit(1);
})