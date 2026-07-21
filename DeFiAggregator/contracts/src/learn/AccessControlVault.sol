// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title AccessControlVault
 * @notice 包含两种访问控制漏洞的合约
 * @dev 漏洞 1: 关键函数缺少 onlyOwner
 * @dev 漏洞 2: 签名验证没有 nonce/chainId 保护，可被重放
 */
contract AccessControlVault {
    address public owner;
    uint256 public fee = 10; // 10 basis points = 0.1%

    // 签名相关
    mapping (bytes => bool) public executedSigs; // 🔴 只存签名本身，没有 nonce

    constructor() payable {
        owner = msg.sender;
    }

    // ====== 漏洞 1: 缺少权限检查 ======

    // 🔴 任何人都能改费率！
    function setFee(uint256 _newFee) external {
        require(_newFee <= 1000, "Fee too high"); // 有上限，但任何人都能调！
        fee = _newFee;
    }

    // 🔴 任何人都能改 owner！
    function changeOwner(address _newOwner) external {
        owner = _newOwner; // 没有 onlyOwner 修饰符！
    }

    // ====== 漏洞 2: 签名重放 ======

    /// @notice 用签名授权提取 ETH（meta-transaction 模式）
    /// @dev 🔴 签名的 hash 不包括 chainId / nonce / 合约地址
    function withdrawBySignature(address to, uint256 amount, bytes calldata signature) external {
        // hash 只有 to 和 amount — 缺少 nonce + chainId + address(this)
        bytes32 hash = keccak256(abi.encodePacked(to, amount));
        bytes32 ethSignedHash = keccak256(abi.encodePacked("\x19Ethereum Signed Message:\n32", hash));

        address signer = _recover(ethSignedHash, signature);
        require(signer == owner, "Invalid signature");
        require(!executedSigs[signature], "Already executed"); // 🔴 仅按完整签名去重

        executedSigs[signature] = true;
        (bool ok, ) = to.call{value:amount}("");
        require(ok, "Transfer failed");
    }

    // ====== 修复版: 带 nonce + chainId + 合约地址 ======

    mapping (address => mapping (uint256 => bool)) public usedNonces;

    function withdrawBySignatureSecure(address to, uint256 amount, uint256 nonce, bytes calldata signature) external {
        require(!usedNonces[owner][nonce], "Nonce used");

        bytes32 hash = keccak256(
            abi.encodePacked(to, amount, nonce, block.chainid, address(this))
            // ✅ 包含 nonce 防重放 + chainId 防跨链 + address(this) 防跨合约
        );
        bytes32 ethSignedHash = keccak256(
            abi.encodePacked("\x19Ethereum Signed Message:\n32", hash)
        );

        address signer = _recover(ethSignedHash, signature);
        require(signer == owner, "Invalid signature");

        usedNonces[owner][nonce] = true;
        (bool ok, ) = to.call{value:amount}("");
        require(ok, "Transfer failed");
    }

    receive() external payable {}

    // ====== ECDSA Recovery ======

    function _recover(bytes32 hash, bytes memory signature) internal pure returns (address) {
        require(signature.length == 65, "Invalid sig length");
        bytes32 r;
        bytes32 s;
        uint8 v;
        assembly {
            r := mload(add(signature, 32))
            s := mload(add(signature, 64))
            v := byte(0, mload(add(signature, 96)))
        }
        if (v < 27) v += 27;
        require(v == 27 || v == 28, "Invalid v");
        return ecrecover(hash, v, r, s);
    }
}