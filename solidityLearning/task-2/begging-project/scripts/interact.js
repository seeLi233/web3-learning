const hre = require("hardhat");

// 填入部署的合约地址
const CONTRACT_ADDRESS = "0x8FE100BCF294cE1585b63AEa639F2bC23f7E66D6";

async function main() {
  // 连接合约
  const beggingContract = await hre.ethers.getContractAt("BeggingContract", CONTRACT_ADDRESS);
  
  // 获取当前账户（部署者和测试账户）
  const [owner, donor1, donor2] = await hre.ethers.getSigners();
  console.log(await owner.address);
  //console.log("当前操作账户：", owner.getAddress(), donor1.getAddress(), donor2.getAddress());

  // 示例1：查询合约余额
  const balance = await beggingContract.getContractBalance();
  console.log(balance);
  console.log("合约当前余额：", balance, "ETH");

  // 示例2：donor1 捐赠 0.1 ETH
  console.log("\n donor1 开始捐赠...");
  const donateTx = await beggingContract.connect(donor1).donate({
    value: hre.ethers.parseEther("0.1") // 捐赠金额
  });
  await donateTx.wait(); // 等待交易确认
  console.log("捐赠完成！交易哈希：", donateTx.hash);

  // 示例3：查询 donor1 的捐赠金额
  const donor1Address = await donor1.address;
  const donor1Donation = await beggingContract.getDonation(donor1Address);
  console.log("donor1 捐赠总额：", donor1Donation, "ETH");

  // 示例4：所有者提取资金（仅在合约有余额时执行）
  if (balance.gt(0)) {
    console.log("\n 所有者开始提取资金...");
    const withdrawTx = await beggingContract.connect(owner).withdraw();
    await withdrawTx.wait();
    console.log("提取完成！交易哈希：", withdrawTx.hash);
  }

  // 示例5：查询排行榜（额外功能）
  const top1 = await beggingContract.topDonors(0);
  const top1Amount = await beggingContract.topDonations(0);
  console.log("\n 捐赠榜首：", top1, "金额：", top1Amount, "ETH");
}

main()
  .then(() => process.exit(0))
  .catch((error) => {
    console.error("交互失败：", error);
    process.exit(1);
  });