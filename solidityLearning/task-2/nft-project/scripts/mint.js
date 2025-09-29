const hre = require("hardhat");
require("dotenv").config();

async function main() {
    // 合约地址
    const contractAddress = "0x428d3bd54Db0b263A5AeAa588A6Ce41D3b1C765B";
    // 接收 NFT 的钱包地址
    const recipient = "0x5DBA8b9595b944ae57Df70B64EEBd09c421e951f";
    // 元数据 IPFS 连接
    const tokenURI = "ipfs://bafkreihsuu4woccsbi43httj52u2xvlzp4jb4u2j7tmsft6n4eagl74bcm";

    // 连接到已部署的合约
    const MyNFT = await hre.ethers.getContractFactory("MyNFT");
    const myNFT = await MyNFT.attach(contractAddress);

    // 调用 mintNFT 函数
    const tx = await myNFT.mintNFT(recipient, tokenURI);
    await tx.wait();

    console.log(`成功上链, 交易成功！交易哈希值:${tx.hash}`);
    console.log()
}

main().catch((error) => {
    console.error(error);
    process.exitCode = 1;
})