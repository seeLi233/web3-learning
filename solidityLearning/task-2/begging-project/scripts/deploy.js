const hre = require("hardhat");

async function main() {
    const BeggingContract = await hre.ethers.getContractFactory("BeggingContract");
    const beggingContract = await BeggingContract.deploy();

    await beggingContract.waitForDeployment();

    console.log(`部署地址在：${await beggingContract.getAddress()}`);
}

main().then(() => process.exit(0)).catch((error) => {
    console.error(error);
    process.exit(1);
});