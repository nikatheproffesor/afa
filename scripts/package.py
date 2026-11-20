#!/usr/bin/env python3
"""Build and verify reproducible, unsigned AFA Windows packages."""

import argparse
import hashlib
import io
import json
import os
from pathlib import Path, PurePosixPath
import re
import struct
import subprocess
import tempfile
import zipfile


ROOT = Path(__file__).resolve().parents[1]
VERSION = "0.2.0-alpha.1"
TARGETS = {"afa.exe": ("./cmd/afa", 3), "AFA-Desktop.exe": ("./cmd/afa-desktop", 2)}
SOURCE_DIRS = (".github", "assets", "cmd", "docs", "internal", "scripts")
ROOT_FILES = (
    ".gitattributes", ".gitignore", "CHANGELOG.md", "CODE_OF_CONDUCT.md", "CONTRIBUTING.md", "LICENSE",
    "PRIVACY.md", "README.md", "SECURITY.md", "THIRD_PARTY_NOTICES.md", "go.mod", "go.sum",
)
DISTRIBUTION_FILES = (
    "README.md", "CHANGELOG.md", "LICENSE", "THIRD_PARTY_NOTICES.md", "PRIVACY.md", "SECURITY.md",
    "CONTRIBUTING.md", "CODE_OF_CONDUCT.md",
)
PROVENANCE = "build-provenance.json"
CHECKSUMS = "SHA256SUMS"
BUILD_TARGET = {"os": "windows", "architecture": "amd64", "microarchitecture": "v1", "cgo_enabled": False}


def sha256(content):
    return hashlib.sha256(content).hexdigest()


def source_files():
    paths = [ROOT / name for name in ROOT_FILES if (ROOT / name).is_file()]
    for name in SOURCE_DIRS:
        paths.extend((ROOT / name).rglob("*"))
    return {
        str(path.relative_to(ROOT).as_posix()): path.read_bytes()
        for path in paths
        if path.is_file() and not path.is_symlink() and path.name != ".DS_Store"
        and "__pycache__" not in path.parts and path.suffix != ".pyc"
    }


def tree_hash(files):
    manifest = "".join(f"{sha256(content)}  {name}\n" for name, content in sorted(files.items()))
    return sha256(manifest.encode())


def git_source():
    def git(*args):
        result = subprocess.run(["git", *args], cwd=ROOT, check=False, capture_output=True, text=True)
        return result.stdout.strip() if result.returncode == 0 else None

    if not (ROOT / ".git").exists():
        return {"commit": None, "working_tree_clean": None}
    try:
        commit = git("rev-parse", "--verify", "HEAD")
        status = git("status", "--porcelain")
    except FileNotFoundError:
        return {"commit": None, "working_tree_clean": None}
    if commit is not None and not re.fullmatch(r"[0-9a-f]{40}|[0-9a-f]{64}", commit):
        raise ValueError("unexpected Git commit metadata")
    return {"commit": commit, "working_tree_clean": None if status is None else status == ""}


def zip_bytes(files):
    buffer = io.BytesIO()
    with zipfile.ZipFile(buffer, "w", compression=zipfile.ZIP_STORED) as archive:
        for name, content in sorted(files.items()):
            entry = zipfile.ZipInfo(name, date_time=(1980, 1, 1, 0, 0, 0))
            entry.create_system = 3
            entry.external_attr = (0o100755 if name.endswith(".exe") else 0o100644) << 16
            archive.writestr(entry, content)
    return buffer.getvalue()


def pe_details(content):
    try:
        offset = struct.unpack_from("<I", content, 0x3C)[0]
        machine = struct.unpack_from("<H", content, offset + 4)[0]
        optional = offset + 24
        magic = struct.unpack_from("<H", content, optional)[0]
        subsystem = struct.unpack_from("<H", content, optional + 68)[0]
        certificate_table = struct.unpack_from("<II", content, optional + 112 + 4 * 8)
    except struct.error as error:
        raise ValueError("invalid Windows PE header") from error
    if content[:2] != b"MZ" or content[offset:offset + 4] != b"PE\0\0" or machine != 0x8664 or magic != 0x20B:
        raise ValueError("expected Windows x64 PE32+ executable")
    return {"subsystem": subsystem, "certificate_table_present": any(certificate_table)}


def go_toolchain():
    match = re.search(r"^toolchain (go\d+\.\d+\.\d+)$", (ROOT / "go.mod").read_text(), re.MULTILINE)
    if not match:
        raise ValueError("go.mod must pin an exact toolchain patch")
    return match[1]


def go_notices(go, packages):
    goroot = Path(go("env", "GOROOT"))
    notices = {"licenses/Go-LICENSE.txt": (goroot / "LICENSE").read_bytes()}
    if (goroot / "PATENTS").is_file():
        notices["licenses/Go-PATENTS.txt"] = (goroot / "PATENTS").read_bytes()
    imports = go("list", "-mod=readonly", "-deps", "-f", "{{.ImportPath}}", *packages)
    vendors = {"/".join(name.split("/")[:4]) for name in imports.splitlines() if name.startswith("vendor/golang.org/x/")}
    for module_path in sorted(vendors):
        directory = goroot / "src" / module_path
        license_file = directory / "LICENSE"
        if not license_file.is_file():
            raise ValueError(f"Go vendored module has no license at {module_path}")
        for name in ("LICENSE", "NOTICE", "PATENTS"):
            path = directory / name
            if path.is_file():
                notices[f"licenses/{module_path.removeprefix('vendor/')}/{name}.txt"] = path.read_bytes()
    return notices


def build(destination, version, cli_only):
    if destination.exists():
        raise ValueError("the output directory already exists")
    if ROOT in destination.parents or destination == ROOT:
        raise ValueError("use an output directory outside the source tree")

    files = source_files()
    git = git_source()
    toolchain = go_toolchain()
    env = dict(os.environ, GOENV="off", GOFLAGS="", GOEXPERIMENT="", GOWORK="off", GOTOOLCHAIN=toolchain,
               GOOS="windows", GOARCH="amd64", GOAMD64="v1", CGO_ENABLED="0")

    def go(*args):
        return subprocess.check_output(["go", *args], cwd=ROOT, env=env, text=True).strip()

    if go("env", "GOVERSION") != toolchain:
        raise ValueError("active Go patch does not match go.mod")
    selected = {"afa.exe": TARGETS["afa.exe"]} if cli_only else TARGETS
    payload = {name: files[name] for name in DISTRIBUTION_FILES}
    payload.update({name: data for name, data in files.items() if name.startswith("docs/")})
    payload.update(go_notices(go, [package for package, _ in selected.values()]))
    commit = git["commit"] if git["working_tree_clean"] else None
    metadata = {
        "schema_version": 1, "version": version, "go_version": toolchain,
        "target": BUILD_TARGET,
        "source": {**git, "tree_sha256": tree_hash(files)},
        "binaries": {},
    }

    with tempfile.TemporaryDirectory(prefix="afa-package-") as scratch:
        for binary, (package, subsystem) in selected.items():
            path = Path(scratch) / binary
            link = ["-s", "-w", f"-X=afa/internal/buildinfo.Version={version}",
                    f"-X=afa/internal/buildinfo.Commit={commit or 'unknown'}"]
            if subsystem == 2:
                link.append("-H=windowsgui")
            flags = ["-mod=readonly", "-trimpath", "-buildvcs=false", "-ldflags=" + " ".join(link)]
            go("build", *flags, "-o", str(path), package)
            content = path.read_bytes()
            details = pe_details(content)
            if details != {"subsystem": subsystem, "certificate_table_present": False}:
                raise ValueError(f"unexpected Windows subsystem or certificate table: {binary}")
            payload[binary] = content
            metadata["binaries"][binary] = {**details, "package": package, "flags": flags, "sha256": sha256(content)}

    if files != source_files() or git != git_source():
        raise ValueError("source changed during the build")

    provenance = (json.dumps(metadata, indent=2, sort_keys=True) + "\n").encode()
    payload[PROVENANCE] = provenance
    prefix = f"afa-{version}"
    artifacts = {
        PROVENANCE: provenance,
        f"{prefix}-windows-x64.zip": zip_bytes(payload),
        f"{prefix}-source.zip": zip_bytes({f"{prefix}/{name}": data for name, data in files.items()}),
    }
    artifacts[CHECKSUMS] = "".join(f"{sha256(data)}  {name}\n" for name, data in sorted(artifacts.items())).encode()
    destination.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="afa-stage-", dir=destination.parent) as scratch:
        stage = Path(scratch) / "delivery"
        stage.mkdir()
        for name, data in artifacts.items():
            (stage / name).write_bytes(data)
        verify(stage)
        # Reserve the final path exclusively, then place validated artifacts without overwriting files.
        destination.mkdir()
        for name in sorted(artifacts):
            with (destination / name).open("xb") as product:
                product.write(artifacts[name])


def read_zip(path):
    with zipfile.ZipFile(path) as archive:
        entries = archive.infolist()
        names = [entry.filename for entry in entries]
        if len(entries) > 4096 or sum(entry.file_size for entry in entries) > (128 << 20):
            raise ValueError(f"ZIP exceeds release size limits: {path.name}")
        if len(names) != len(set(names)) or any(entry.compress_type != zipfile.ZIP_STORED for entry in entries):
            raise ValueError(f"invalid ZIP entries: {path.name}")
        for name in names:
            path_name = PurePosixPath(name)
            if path_name.is_absolute() or ".." in path_name.parts or "\\" in name:
                raise ValueError(f"unsafe ZIP entry in {path.name}")
        return {name: archive.read(name) for name in names}


def valid_version(version):
    return isinstance(version, str) and bool(re.fullmatch(
        r"[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?", version,
    ))


def verify(directory):
    provenance = (directory / PROVENANCE).read_bytes()
    metadata = json.loads(provenance)
    if metadata["schema_version"] != 1 or not valid_version(metadata["version"]):
        raise ValueError("unsupported provenance metadata")
    if metadata["target"] != BUILD_TARGET or not re.fullmatch(r"go\d+\.\d+\.\d+", metadata["go_version"]):
        raise ValueError("unsupported build target or Go version")
    prefix = f"afa-{metadata['version']}"
    portable_name, source_name = f"{prefix}-windows-x64.zip", f"{prefix}-source.zip"
    names = (PROVENANCE, portable_name, source_name)
    expected = "".join(f"{sha256((directory / name).read_bytes())}  {name}\n" for name in sorted(names))
    if (directory / CHECKSUMS).read_bytes() != expected.encode():
        raise ValueError("SHA-256 verification failed")

    portable = read_zip(directory / portable_name)
    if portable[PROVENANCE] != provenance:
        raise ValueError("bundled provenance does not match")
    if "licenses/Go-LICENSE.txt" not in portable or any(name not in portable for name in DISTRIBUTION_FILES):
        raise ValueError("portable ZIP is missing required notices")
    binaries = metadata["binaries"]
    if "afa.exe" not in binaries or any(name not in TARGETS for name in binaries):
        raise ValueError("unexpected executable manifest")
    if set(name for name in portable if name.lower().endswith(".exe")) != set(binaries):
        raise ValueError("portable ZIP executables do not match the manifest")
    for binary, info in binaries.items():
        observed = pe_details(portable[binary])
        package, subsystem = TARGETS[binary]
        wanted = {"subsystem": subsystem, "certificate_table_present": False}
        if (sha256(portable[binary]) != info["sha256"] or observed != wanted or info["package"] != package
                or any(info[key] != value for key, value in wanted.items())):
            raise ValueError(f"executable validation failed: {binary}")

    contents = read_zip(directory / source_name)
    if any(not name.startswith(prefix + "/") for name in contents):
        raise ValueError("unexpected source ZIP structure")
    files = {name.split("/", 1)[1]: data for name, data in contents.items()}
    if tree_hash(files) != metadata["source"]["tree_sha256"]:
        raise ValueError("source snapshot digest mismatch")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    modes = parser.add_mutually_exclusive_group(required=True)
    modes.add_argument("--out", type=Path, help="new directory to receive the release files")
    modes.add_argument("--verify", type=Path, help="existing release directory to validate")
    parser.add_argument("--version", default=VERSION, help="version for the built binaries and archives")
    parser.add_argument("--cli-only", action="store_true", help="package only the CLI for development")
    options = parser.parse_args()
    if not valid_version(options.version):
        parser.error("expected a numeric version with an optional prerelease suffix")
    if options.verify and (options.cli_only or options.version != VERSION):
        parser.error("--verify reads the version and executables from the package metadata")
    try:
        if options.out:
            build(options.out.resolve(), options.version, options.cli_only)
            verify(options.out)
        else:
            verify(options.verify)
    except (OSError, ValueError, KeyError, TypeError, subprocess.CalledProcessError, zipfile.BadZipFile) as error:
        parser.exit(1, f"package: {error}\n")
    print(f"Package validation passed: {options.out or options.verify}")


if __name__ == "__main__":
    main()
