const hre = require("hardhat");

async function main() {
    // 部署合约
    const MyNFT = await hre.ethers.getContractFactory("MyNFT");

    // 设置部署者地址
    const [deployer] = await hre.ethers.getSigners();
    console.log(`部署者地址：${deployer.address}`);


    const myNFT = await MyNFT.deploy("MyNFT", "CAT");
    await myNFT.waitForDeployment();

    const contractAddress = await myNFT.getAddress();
    console.log(`NFT合约以部署到:${contractAddress}`);
}

// 执行部署
main().then(()=> process.exit(0)).catch((error)=> {
    console.error(error);
    process.exit(1);
});