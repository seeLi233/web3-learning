const hre = require("hardhat");

async function main() {
    // 获取合约工厂
    const MyERC20 = await hre.ethers.getContractFactory("MyERC20");

    // 部署合约，参数：名称、符号、初始供应商
    const token = await MyERC20.deploy("MyToken", "MTK", 10000);
    await token.waitForDeployment();

    const contractAddress = await token.getAddress();
    console.log("MyERC20 deployed to:", contractAddress);

    // 验证合约（如果在支持验证的网络上）
    if(hre.network.name !== "hardhat" && hre.network.name !== "localhost") {
        console.log("Waiting for block confirmations...");
        // 等待6个区块确认
        // await token.deployTransaction().wait(6);

        console.log("Verifying contract on Etherscan...");
        await hre.run("verify:verify", {
            address: contractAddress,
            constructorArguments: ["MyToken", "MTK", 10000],
        });
        console.log("Contract verified on Etherscan");
    }
}

// 执行部署
main().then(() => process.exit(0)).catch((error) => {
    console.error(error);
    process.exit(1);
});