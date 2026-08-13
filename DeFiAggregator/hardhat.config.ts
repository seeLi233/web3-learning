import "dotenv/config";
import hardhatToolboxMochaEthersPlugin from "@nomicfoundation/hardhat-toolbox-mocha-ethers";
import { defineConfig } from "hardhat/config";

export default defineConfig({
  plugins: [hardhatToolboxMochaEthersPlugin],
  solidity: {
    profiles: {
      default: {
        version: "0.8.28",
        settings: {
          optimizer: {
            enabled: true,
            runs: 1,  // 工厂合约不频繁调用，最小化部署字节码
          },
          viaIR: true,
        },
      },
      production: {
        version: "0.8.28",
        settings: {
          optimizer: {
            enabled: true,
            runs: 200,
          },
        },
      },
    },
  },
  networks: {
    default: {
      type: "edr-simulated",
      chainType: "l1",
      allowUnlimitedContractSize: true,
    },
    hardhat: {
      type: "edr-simulated",
      chainType: "l1",
      allowUnlimitedContractSize: true,
    },
    hardhatMainnet: {
      type: "edr-simulated",
      chainType: "l1",
    },
    hardhatOp: {
      type: "edr-simulated",
      chainType: "op",
    },
    sepolia: {
      type: "http",
      chainType: "l1",
      url: process.env.SEPOLIA_RPC_URL!,
      accounts: [process.env.SEPOLIA_PRIVATE_KEY!],
    },
    // ============ 新增：主网 Fork 网络 ============
    // 为什么 type 是 "edr-simulated" 而不是 "http"？
    // → fork 网络是本地模拟的（EDR = Ethereum Dev Runtime），
    //    它只是在需要读主网数据时才向远程 RPC 发请求，其余都在本地跑
    mainnetFork: {
      type: "edr-simulated",    // 本地模拟网络
      chainType: "l1",
      forking: {
        // 为什么用 process.env 而不是硬编码？
        // → RPC 地址含 API key，不能提交到 git，从 .env 读取
        url: process.env.MAINNET_RPC_URL!,

        // 为什么不固定 blockNumber？（默认 fork 最新块）
        // → 固定旧块号需要 archive 归档节点，而免费 Infura 只支持 fork
        //    最近 ~128 个块内的状态，fork 旧块会报 "missing trie node"。
        //    本测试只断言 decimals()（不可变常量），fork 最新块同样稳定，
        //    所以不必固定块号。若日后要测余额/价格这类会变的状态，
        //    再换 archive RPC 并固定 blockNumber 以保可复现。
      }
    }
  },
});
