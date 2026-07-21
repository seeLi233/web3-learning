"""
Slither 审计脚本 — 兼容 Hardhat 3 的分割 build-info 格式
解决问题:
  1. Hardhat 3 把 build-info 拆成 input/output 两个 JSON 文件
  2. Hardhat 3 使用虚拟路径 (project/ 和 npm/ 前缀) 而非真实文件路径
运行方式: python slither_audit.py
"""
import json
import os
import re
import sys
from pathlib import Path

# ============================================================
# Step 1: Monkey-patch 1 — 修复路径解析
# Hardhat 3 使用虚拟路径:
#   project/contracts/... -> contracts/...
#   npm/@openzeppelin/contracts@5.6.1/... -> node_modules/@openzeppelin/contracts/...
# ============================================================
import crytic_compile.utils.naming as naming_module

original_verify = naming_module._verify_filename_existence

def patched_verify_filename_existence(filename, cwd):
    """扩展的文件路径解析，兼容 Hardhat 3 虚拟路径"""
    fn = Path(filename)
    fn_str = str(filename)

    # Hardhat 3 project/ 前缀 -> 去掉前缀
    if fn_str.startswith("project/") or fn_str.startswith("project\\"):
        mapped = Path(str(fn).split("/", 1)[1] if "/" in str(fn) else str(fn).split("\\", 1)[1])
        # Try mapped path directly
        if mapped.exists():
            return mapped
        if cwd.joinpath(mapped).exists():
            return cwd.joinpath(mapped)

    # Hardhat 3 npm/ 前缀 -> node_modules/
    # npm/@openzeppelin/contracts@5.6.1/... -> node_modules/@openzeppelin/contracts/...
    npm_match = re.match(r'^npm[/\\](.+?)@[\d.]+[/\\](.*)', fn_str)
    if npm_match:
        pkg = npm_match.group(1)   # e.g., @openzeppelin/contracts
        rest = npm_match.group(2)  # e.g., token/ERC20/ERC20.sol
        mapped = Path("node_modules") / pkg / rest
        if mapped.exists():
            return mapped
        if cwd.joinpath(mapped).exists():
            return cwd.joinpath(mapped)

    # Try the original function
    return original_verify(filename, cwd)


naming_module._verify_filename_existence = patched_verify_filename_existence
print("[Step 1a] Patched _verify_filename_existence for Hardhat 3 virtual paths")


# ============================================================
# Step 1b: Monkey-patch 2 — 修复 Hardhat 3 分割 build-info
# ============================================================
import crytic_compile.platform.hardhat as hh_module

def patched_hardhat_like_parsing(crytic_compile, target, build_directory, working_dir):
    """与原始函数相同，但兼容 Hardhat 3 的 split build-info 格式"""
    from crytic_compile.compilation_unit import CompilationUnit, CompilerVersion
    from crytic_compile.utils.natspec import Natspec
    from crytic_compile.utils.naming import extract_name, convert_filename
    from crytic_compile.platform.solc import relative_to_short
    from crytic_compile.platform.exceptions import InvalidCompilation

    build_dir = Path(build_directory)
    if not build_dir.is_dir():
        raise InvalidCompilation(f"Compilation failed. {build_directory} is not a directory.")

    files = sorted(
        os.listdir(build_dir), key=lambda x: os.path.getmtime(Path(build_dir, x))
    )
    files = [str(f) for f in files if str(f).endswith(".json")]
    if not files:
        raise InvalidCompilation(f"Compilation failed. {build_directory} is empty.")

    for file in files:
        build_info = Path(build_dir, file)

        # Skip .output.json files (merged when loading the main file)
        if file.endswith(".output.json"):
            continue

        uniq_id = file if ".json" not in file else file[0:-5]

        with open(build_info, encoding="utf8") as file_desc:
            loaded_json = json.load(file_desc)

            # Hardhat 3 fix: merge output from .output.json if not in main file
            if "output" not in loaded_json:
                output_file = Path(build_dir, file.replace(".json", ".output.json"))
                if output_file.exists():
                    with open(output_file, encoding="utf8") as of:
                        output_data = json.load(of)
                    loaded_json["output"] = output_data["output"]
                else:
                    raise InvalidCompilation(
                        f"No 'output' found in {file} and {output_file} does not exist."
                    )

            compilation_unit = CompilationUnit(crytic_compile, uniq_id)

            targets_json = loaded_json["output"]
            version_from_config = loaded_json["solcVersion"]
            input_json = loaded_json["input"]
            compiler = "solc" if input_json["language"] == "Solidity" else "vyper"
            optimized = input_json["settings"]["optimizer"].get("enabled", False)

            compilation_unit.compiler_version = CompilerVersion(
                compiler=compiler, version=version_from_config, optimized=optimized
            )

            skip_filename = compilation_unit.compiler_version.version in [
                f"0.4.{x}" for x in range(0, 10)
            ]

            if "sources" in targets_json:
                for path, info in targets_json["sources"].items():
                    if skip_filename:
                        path = convert_filename(
                            target, relative_to_short, crytic_compile, working_dir=working_dir
                        )
                    else:
                        path = convert_filename(
                            path, relative_to_short, crytic_compile, working_dir=working_dir
                        )
                    source_unit = compilation_unit.create_source_unit(path)
                    source_unit.ast = info.get("ast", info.get("legacyAST"))
                    if source_unit.ast is None:
                        raise InvalidCompilation(f"AST not found for {path} in {build_info}")

            if "contracts" in targets_json:
                for original_filename, contracts_info in targets_json["contracts"].items():
                    filename = convert_filename(
                        original_filename, relative_to_short, crytic_compile, working_dir=working_dir
                    )
                    source_unit = compilation_unit.create_source_unit(filename)

                    for original_contract_name, info in contracts_info.items():
                        contract_name = extract_name(original_contract_name)
                        source_unit.add_contract_name(contract_name)
                        compilation_unit.filename_to_contracts[filename].add(contract_name)
                        source_unit.abis[contract_name] = info["abi"]
                        source_unit.bytecodes_init[contract_name] = info["evm"]["bytecode"]["object"]
                        source_unit.bytecodes_runtime[contract_name] = info["evm"]["deployedBytecode"]["object"]
                        source_unit.srcmaps_init[contract_name] = info["evm"]["bytecode"]["sourceMap"].split(";")
                        source_unit.srcmaps_runtime[contract_name] = info["evm"]["deployedBytecode"]["sourceMap"].split(";")
                        userdoc = info.get("userdoc", {})
                        devdoc = info.get("devdoc", {})
                        natspec = Natspec(userdoc, devdoc)
                        source_unit.natspec[contract_name] = natspec


hh_module.hardhat_like_parsing = patched_hardhat_like_parsing
print("[Step 1b] Patched hardhat_like_parsing for Hardhat 3 split build-info format")


# ============================================================
# Step 2: 手动 Hardhat 编译（确保 artifacts 存在）
# ============================================================
print("[Step 2] Compiling with Hardhat...")
import subprocess
result = subprocess.run(
    "npx hardhat compile --force",
    cwd=r"D:\web3-learning\DeFiAggregator",
    capture_output=True, text=True,
    shell=True
)
if result.returncode != 0:
    print("Hardhat compilation FAILED:")
    print(result.stderr)
    sys.exit(1)
print("[Step 2] Hardhat compilation succeeded")

# ============================================================
# Step 3: 用 crytic-compile 加载编译产物
# ============================================================
print("[Step 3] Loading compilation with crytic-compile...")
from crytic_compile import CryticCompile

compilation = CryticCompile(
    r"D:\web3-learning\DeFiAggregator",
    compile_force_framework="hardhat",
    ignore_compile=True,  # Skip clean + compile, parse existing artifacts
)

total_contracts = sum(len(cu.filename_to_contracts) for cu in compilation.compilation_units.values())
print(f"[Step 3] Loaded {len(compilation.compilation_units)} compilation units, {total_contracts} source files")

# ============================================================
# Step 4: 初始化 Slither，注册所有检测器
# ============================================================
print("[Step 4] Initializing Slither and registering detectors...")
from slither import Slither
from slither.__main__ import get_detectors_and_printers

slither = Slither(compilation)

# 注册所有 101 个 Slither 检测器
all_detectors, _ = get_detectors_and_printers()
for detector_cls in all_detectors:
    slither.register_detector(detector_cls)

# 只分析与 DeFiDex 相关的发现
for cu in compilation.compilation_units.values():
    for filename in cu.filenames:
        if "DeFiDex" in str(filename.absolute):
            slither.add_path_to_filter(str(filename.absolute))

contract_names = [c.name for c in slither.contracts]
defidex_contracts = [c for c in slither.contracts if "DeFiDex" in c.name]
print(f"[Step 4] Registered {len(slither.detectors)} detectors")
print(f"[Step 4] DeFiDex contract found: {[c.name for c in defidex_contracts]}")

# ============================================================
# Step 5: 运行所有检测器并输出结果
# ============================================================
print()
print("=" * 70)
print("SLITHER STATIC ANALYSIS REPORT — DeFiDex.sol")
print("=" * 70)

total_issues = 0
detectors_run = 0

for detector in slither.detectors:
    detectors_run += 1
    try:
        results = detector.detect()
    except Exception:
        continue

    # 只保留与 DeFiDex.sol 相关的发现（精确匹配文件路径）
    defidex_results = []
    for r in results:
        elems = r.get("elements", [])
        is_defidex = False
        for elem in elems:
            fname = ""
            try:
                fname = str(elem.source_mapping.filename.relative)
            except AttributeError:
                if isinstance(elem, dict):
                    fname = elem.get("source_mapping", {}).get("filename_relative", "")
            # 精确匹配: 文件名以 /src/dex/DeFiDex.sol 或 \src\dex\DeFiDex.sol 结尾
            if fname.endswith("src/dex/DeFiDex.sol") or fname.endswith("src\\dex\\DeFiDex.sol"):
                is_defidex = True
                break
        if is_defidex:
            defidex_results.append(r)

    if defidex_results:
        total_issues += len(defidex_results)
        print()
        print(f"[{detector.ARGUMENT}] {detector.WIKI_TITLE}")
        print(f"  Impact    : {detector.IMPACT}")
        print(f"  Confidence: {detector.CONFIDENCE}")
        print(f"  Description: {detector.WIKI_DESCRIPTION}")
        print(f"  Findings  : {len(defidex_results)}")
        for i, r in enumerate(defidex_results):
            print(f"  --- Finding #{i+1} ---")
            print(f"  Check: {r.get('check', 'N/A')}")
            elems = r.get("elements", [])
            for elem in elems:
                try:
                    src = elem.source_mapping
                    loc = f"{src.filename.relative}:{src.lines}"
                    print(f"    Location: {loc}")
                    name = getattr(elem, 'name', None) or getattr(elem, 'canonical_name', None)
                    if name:
                        print(f"    Element : {name}")
                except AttributeError:
                    if isinstance(elem, dict):
                        sm = elem.get("source_mapping", {})
                        loc = f"{sm.get('filename_relative', '?')}:{sm.get('lines', '?')}"
                        print(f"    Location: {loc}")
                        print(f"    Element : {elem.get('name', elem.get('type', '?'))} ({elem.get('type', '?')})")
            if "description" in r:
                print(f"  Details: {r['description']}")
        print()

print()
print("=" * 70)
print(f"SUMMARY: {detectors_run} detectors run, {total_issues} issues found")
print("=" * 70)
