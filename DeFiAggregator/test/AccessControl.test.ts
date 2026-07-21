import { expect } from "chai";
import { network } from "hardhat";

const { ethers } = await network.create();

describe("AccessControl", function () {
    let access: any;
    let owner: any;
    let admin: any;
    let moderator: any;
    let user: any;
    let stranger: any;

    const ADMIN_ROLE = ethers.keccak256(ethers.toUtf8Bytes("ADMIN_ROLE"));
    const MODERATOR_ROLE = ethers.keccak256(ethers.toUtf8Bytes("MODERATOR_ROLE"));
    const USER_ROLE = ethers.keccak256(ethers.toUtf8Bytes("USER_ROLE"));

    beforeEach(async function () {
        [owner, admin, moderator, user, stranger] = await ethers.getSigners();
        access = await ethers.deployContract("AccessControl");
    });

    describe("部署", function () {
        it("owner 应该是 deployer", async function () {
            expect(await access.owner()).to.equal(owner.address);
        });

        it("owner 应该有 ADMIN_ROLE", async function () {
            expect(await access.hasRole(ADMIN_ROLE, owner.address)).to.be.true;
        });

        it("owner 拥有 DEFAULT_ADMIN_ROLE (0x00)", async function () {
            const DEFAULT_ADMIN_ROLE = "0x0000000000000000000000000000000000000000000000000000000000000000";
            expect(await access.hasRole(DEFAULT_ADMIN_ROLE, owner.address)).to.be.true;
        });

        it("非 owner 不应该有 ADMIN_ROLE", async function () {
            expect(await access.hasRole(ADMIN_ROLE, stranger.address)).to.be.false;
        });
    });

    describe("修饰符: onlyOwner", function () {
        it("owner 可以授权角色", async function () {
            await access.grantRole(ADMIN_ROLE, admin.address);
            expect(await access.hasRole(ADMIN_ROLE, admin.address)).to.be.true;
        });

        it("非 owner 不能授权角色", async function () {
            await expect(
                access.connect(stranger).grantRole(USER_ROLE, user.address)
            ).to.be.revertedWithCustomError(access, "NotOwner");
        });

        it("非 owner 不能撤销角色", async function () {
            // owner 先授予角色
            await access.grantRole(USER_ROLE, user.address);
            // stranger 尝试撤销 → 应 revert NotOwner
            await expect(
                access.connect(stranger).revokeRole(USER_ROLE, user.address)
            ).to.be.revertedWithCustomError(access, "NotOwner");
        });

        it("非 owner 不能转移 ownership", async function () {
            await expect(
                access.connect(stranger).transferOwnership(stranger.address)
            ).to.be.revertedWithCustomError(access, "NotOwner");
        });
    });

    describe("修饰符: onlyRole", function () {
        beforeEach(async function () {
            await access.grantRole(ADMIN_ROLE, admin.address);
            await access.grantRole(USER_ROLE, user.address);
        });

        it("admin 可以执行 adminAction (ADMIN_ROLE)", async function () {
            await expect(access.connect(admin).adminAction("test"))
                .to.emit(access, "ActionPerformed")
                .withArgs(admin.address, "test");
        });

        it("user 不能执行 adminAction (没有 ADMIN_ROLE)", async function () {
            await expect(
                access.connect(user).adminAction("hack")
            ).to.be.revertedWithCustomError(access, "NotAuthorized");
        });

        it("moderator 可以执行 moderatorAction (MODERATOR_ROLE)", async function () {
            await access.grantRole(MODERATOR_ROLE, moderator.address);
            await expect(access.connect(moderator).moderatorAction("moderate"))
                .to.emit(access, "ActionPerformed")
                .withArgs(moderator.address, "moderate");
        });

        it("无角色者不能执行 moderatorAction", async function () {
            await expect(
                access.connect(stranger).moderatorAction("hack")
            ).to.be.revertedWithCustomError(access, "NotAuthorized");
        });

        it("user 可以执行 userAction (USER_ROLE)", async function () {
            await expect(access.connect(user).userAction("post"))
                .to.emit(access, "ActionPerformed")
                .withArgs(user.address, "post");
        });

        it("无角色者不能执行 userAction", async function () {
            await expect(
                access.connect(stranger).userAction("spam")
            ).to.be.revertedWithCustomError(access, "NotAuthorized");
        });
    });

    describe("角色管理", function () {
        it("应该正确授予角色", async function () {
            await access.grantRole(MODERATOR_ROLE, moderator.address);
            expect(await access.hasRole(MODERATOR_ROLE, moderator.address)).to.be.true;
        });

        it("重复授予角色应该失败", async function () {
            await access.grantRole(USER_ROLE, user.address);
            await expect(
                access.grantRole(USER_ROLE, user.address)
            ).to.be.revertedWithCustomError(access, "RoleAlreadyGranted");
        });

        it("应该正确撤销角色", async function () {
            await access.grantRole(USER_ROLE, user.address);
            await access.revokeRole(USER_ROLE, user.address);
            expect(await access.hasRole(USER_ROLE, user.address)).to.be.false;
        });

        it("撤销未授予的角色应该 revert RoleNotGranted", async function () {
            // user 没有 MODERATOR_ROLE → revokeRole 应该 revert RoleNotGranted
            await expect(
                access.revokeRole(MODERATOR_ROLE, user.address)
            ).to.be.revertedWithCustomError(access, "RoleNotGranted");
        });

        it("不能撤销自己的角色", async function () {
            // owner 尝试撤销自己的 ADMIN_ROLE → CannotRevokeSelf
            await expect(
                access.revokeRole(ADMIN_ROLE, owner.address)
            ).to.be.revertedWithCustomError(access, "CannotRevokeSelf");
        });

        it("可以放弃自己的角色", async function () {
            await access.grantRole(USER_ROLE, user.address);
            await access.connect(user).renounceRole(USER_ROLE);
            expect(await access.hasRole(USER_ROLE, user.address)).to.be.false;
        });

        it("放弃未拥有的角色应该 revert RoleNotGranted", async function () {
            // user 没有 MODERATOR_ROLE → renounceRole 应该 revert
            await expect(
                access.connect(user).renounceRole(MODERATOR_ROLE)
            ).to.be.revertedWithCustomError(access, "RoleNotGranted");
        });
    });

    describe("零地址防护", function () {
        it("不能授予零地址", async function () {
            await expect(
                access.grantRole(USER_ROLE, ethers.ZeroAddress)
            ).to.be.revertedWithCustomError(access, "ZeroAddress");
        });

        it("不能撤销零地址的角色", async function () {
            await expect(
                access.revokeRole(USER_ROLE, ethers.ZeroAddress)
            ).to.be.revertedWithCustomError(access, "ZeroAddress");
        });

        it("不能转移 ownership 到零地址", async function () {
            await expect(
                access.transferOwnership(ethers.ZeroAddress)
            ).to.be.revertedWithCustomError(access, "ZeroAddress");
        });
    });

    describe("事件", function () {
        it("授予角色应该 emit RoleGranted", async function () {
            await expect(access.grantRole(USER_ROLE, user.address))
                .to.emit(access, "RoleGranted")
                .withArgs(USER_ROLE, user.address, owner.address);
        });

        it("撤销角色应该 emit RoleRevoked", async function () {
            await access.grantRole(USER_ROLE, user.address);
            await expect(access.revokeRole(USER_ROLE, user.address))
                .to.emit(access, "RoleRevoked")
                .withArgs(USER_ROLE, user.address, owner.address);
        });

        it("放弃角色应该 emit RoleRevoked", async function () {
            await access.grantRole(USER_ROLE, user.address);
            await expect(access.connect(user).renounceRole(USER_ROLE))
                .to.emit(access, "RoleRevoked")
                .withArgs(USER_ROLE, user.address, user.address);
        });

        it("转移 ownership 应该 emit OwnerChanged", async function () {
            await expect(access.transferOwnership(admin.address))
                .to.emit(access, "OwnerChanged")
                .withArgs(owner.address, admin.address);
        });
    });

    describe("转移 ownership", function () {
        it("应该更新 owner", async function () {
            await access.transferOwnership(admin.address);
            expect(await access.owner()).to.equal(admin.address);
        });

        it("新 owner 获得 ADMIN_ROLE", async function () {
            await access.transferOwnership(admin.address);
            expect(await access.hasRole(ADMIN_ROLE, admin.address)).to.be.true;
        });

        it("旧 owner 失去 ADMIN_ROLE", async function () {
            await access.transferOwnership(admin.address);
            expect(await access.hasRole(ADMIN_ROLE, owner.address)).to.be.false;
        });

        it("旧 owner 失去 owner 身份", async function () {
            await access.transferOwnership(admin.address);
            expect(await access.owner()).to.not.equal(owner.address);
        });

        it("新 owner 可以行使 owner 权限", async function () {
            await access.transferOwnership(admin.address);
            // 新 owner (admin) 现在可以授权角色
            await access.connect(admin).grantRole(USER_ROLE, user.address);
            expect(await access.hasRole(USER_ROLE, user.address)).to.be.true;
        });
    });
});
