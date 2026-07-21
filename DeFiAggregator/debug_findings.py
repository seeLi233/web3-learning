"""Quick debug: check which detectors find DeFiDex issues"""
import json, os, re, sys
from pathlib import Path

# Patch 1: file resolution
import crytic_compile.utils.naming as naming_module
original_verify = naming_module._verify_filename_existence

def patched_verify(filename, cwd):
    fn_str = str(filename)
    # project/ -> strip prefix
    if fn_str.startswith("project/") or fn_str.startswith("project\\"):
        mapped = Path(fn_str.split("/", 1)[1] if "/" in fn_str else fn_str.split("\\", 1)[1])
        if mapped.exists():
            return mapped
        if Path(cwd).joinpath(mapped).exists():
            return Path(cwd).joinpath(mapped)
    # npm/ -> node_modules/
    npm_match = re.match(r'^npm[/\\](.+?)@[\d.]+[/\\](.*)', fn_str)
    if npm_match:
        pkg, rest = npm_match.group(1), npm_match.group(2)
        mapped = Path("node_modules") / pkg / rest
        if mapped.exists():
            return mapped
        if Path(cwd).joinpath(mapped).exists():
            return Path(cwd).joinpath(mapped)
    return original_verify(filename, cwd)

naming_module._verify_filename_existence = patched_verify

# Patch 2: hardhat split build-info
import crytic_compile.platform.hardhat as hh_module

def patched_parsing(crytic_compile, target, build_directory, working_dir):
    from crytic_compile.compilation_unit import CompilationUnit, CompilerVersion
    from crytic_compile.utils.natspec import Natspec
    from crytic_compile.utils.naming import extract_name, convert_filename
    from crytic_compile.platform.solc import relative_to_short
    from crytic_compile.platform.exceptions import InvalidCompilation

    build_dir = Path(build_directory)
    if not build_dir.is_dir():
        raise InvalidCompilation("Not a dir")
    files = sorted(os.listdir(build_dir), key=lambda x: os.path.getmtime(Path(build_dir, x)))
    files = [str(f) for f in files if str(f).endswith(".json")]
    if not files:
        raise InvalidCompilation("Empty")

    for file in files:
        if file.endswith(".output.json"):
            continue
        build_info = Path(build_dir, file)
        uniq_id = file[0:-5]
        with open(build_info, encoding="utf8") as f:
            loaded_json = json.load(f)
            if "output" not in loaded_json:
                output_file = Path(build_dir, file.replace(".json", ".output.json"))
                if output_file.exists():
                    with open(output_file, encoding="utf8") as of:
                        output_data = json.load(of)
                    loaded_json["output"] = output_data["output"]
                else:
                    raise InvalidCompilation("No output file")

            compilation_unit = CompilationUnit(crytic_compile, uniq_id)
            targets_json = loaded_json["output"]
            version_from_config = loaded_json["solcVersion"]
            input_json = loaded_json["input"]
            compiler = "solc" if input_json["language"] == "Solidity" else "vyper"
            optimized = input_json["settings"]["optimizer"].get("enabled", False)
            compilation_unit.compiler_version = CompilerVersion(
                compiler=compiler, version=version_from_config, optimized=optimized
            )
            skip = compilation_unit.compiler_version.version in [f"0.4.{x}" for x in range(0, 10)]

            if "sources" in targets_json:
                for path, info in targets_json["sources"].items():
                    path = convert_filename(
                        target if skip else path, relative_to_short, crytic_compile, working_dir=working_dir
                    )
                    su = compilation_unit.create_source_unit(path)
                    su.ast = info.get("ast", info.get("legacyAST"))

            if "contracts" in targets_json:
                for ofn, contracts_info in targets_json["contracts"].items():
                    filename = convert_filename(ofn, relative_to_short, crytic_compile, working_dir=working_dir)
                    su = compilation_unit.create_source_unit(filename)
                    for ocn, info in contracts_info.items():
                        cn = extract_name(ocn)
                        su.add_contract_name(cn)
                        compilation_unit.filename_to_contracts[filename].add(cn)
                        su.abis[cn] = info["abi"]
                        su.bytecodes_init[cn] = info["evm"]["bytecode"]["object"]
                        su.bytecodes_runtime[cn] = info["evm"]["deployedBytecode"]["object"]
                        su.srcmaps_init[cn] = info["evm"]["bytecode"]["sourceMap"].split(";")
                        su.srcmaps_runtime[cn] = info["evm"]["deployedBytecode"]["sourceMap"].split(";")
                        su.natspec[cn] = Natspec(info.get("userdoc", {}), info.get("devdoc", {}))

hh_module.hardhat_like_parsing = patched_parsing

# Load compilation and slither
from crytic_compile import CryticCompile
from slither import Slither
from slither.__main__ import get_detectors_and_printers

compilation = CryticCompile(
    r"D:\web3-learning\DeFiAggregator",
    compile_force_framework="hardhat",
    ignore_compile=True,
)
slither = Slither(compilation)
all_detectors, _ = get_detectors_and_printers()
for dc in all_detectors:
    slither.register_detector(dc)

# Check each detector for DeFiDex findings
print("Checking detectors for DeFiDex findings...")
print()
for d in slither.detectors:
    try:
        results = d.detect()
    except Exception:
        continue
    in_defidex = False
    for r in results:
        for elem in r.get("elements", []):
            fname = ""
            if isinstance(elem, dict):
                fname = elem.get("source_mapping", {}).get("filename_relative", "")
            else:
                try:
                    fname = str(elem.source_mapping.filename.relative)
                except Exception:
                    pass
            if "DeFiDex" in fname:
                in_defidex = True
                break
        if in_defidex:
            break
    if in_defidex:
        print(f"  [DEFIDEX] {d.ARGUMENT}: {len(results)} findings")
    else:
        print(f"  [OTHER]   {d.ARGUMENT}: {len(results)} findings")
