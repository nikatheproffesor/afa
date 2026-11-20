import json
from pathlib import Path
import struct
import tempfile
import unittest

import package


def synthetic_pe(subsystem):
    # This minimal header is a unit test fixture, never a release executable.
    offset = 64
    optional = offset + 24
    data = bytearray(optional + 240)
    data[:2] = b"MZ"
    struct.pack_into("<I", data, 0x3C, offset)
    data[offset:offset + 4] = b"PE\0\0"
    struct.pack_into("<H", data, offset + 4, 0x8664)
    struct.pack_into("<H", data, optional, 0x20B)
    struct.pack_into("<H", data, optional + 68, subsystem)
    return bytes(data)


class PackageTests(unittest.TestCase):
    def test_stable_archives(self):
        self.assertEqual(package.zip_bytes({"b": b"B", "a": b"A"}),
                         package.zip_bytes({"a": b"A", "b": b"B"}))

    def test_pe_metadata(self):
        self.assertEqual(package.pe_details(synthetic_pe(2)),
                         {"subsystem": 2, "certificate_table_present": False})
        with self.assertRaisesRegex(ValueError, "PE header"):
            package.pe_details(b"truncated")

    def test_zip_rejects_unsafe_names(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "source.zip"
            path.write_bytes(package.zip_bytes({"../example": b"fixture"}))
            with self.assertRaisesRegex(ValueError, "unsafe ZIP entry"):
                package.read_zip(path)

    def test_verify_tampered_package(self):
        binary = synthetic_pe(3)
        sources = {"README.md": b"fixture"}
        metadata = {
            "schema_version": 1,
            "version": package.VERSION,
            "target": package.BUILD_TARGET,
            "go_version": "go1.27.4",
            "source": {"tree_sha256": package.tree_hash(sources)},
            "binaries": {"afa.exe": {"subsystem": 3, "certificate_table_present": False,
                                     "package": "./cmd/afa",
                                     "sha256": package.sha256(binary)}},
        }
        provenance = json.dumps(metadata).encode()
        payload = {name: b"fixture" for name in package.DISTRIBUTION_FILES}
        payload.update({package.PROVENANCE: provenance, "afa.exe": binary, "licenses/Go-LICENSE.txt": b"fixture"})
        prefix = f"afa-{package.VERSION}"

        with tempfile.TemporaryDirectory() as directory:
            directory = Path(directory)
            portable = directory / f"{prefix}-windows-x64.zip"
            files = {
                package.PROVENANCE: provenance,
                portable.name: package.zip_bytes(payload),
                f"{prefix}-source.zip": package.zip_bytes({f"{prefix}/{name}": data for name, data in sources.items()}),
            }

            def manifest():
                return "".join(f"{package.sha256(data)}  {name}\n" for name, data in sorted(files.items()))

            for name, data in files.items():
                (directory / name).write_bytes(data)
            checksums = directory / package.CHECKSUMS
            checksums.write_bytes(manifest().encode("ascii"))
            package.verify(directory)

            files[portable.name] += b"modified"
            portable.write_bytes(files[portable.name])
            with self.assertRaisesRegex(ValueError, "SHA-256 verification failed"):
                package.verify(directory)

            payload["afa.exe"] = synthetic_pe(2)
            files[portable.name] = package.zip_bytes(payload)
            portable.write_bytes(files[portable.name])
            checksums.write_bytes(manifest().encode("ascii"))
            with self.assertRaisesRegex(ValueError, "executable validation failed"):
                package.verify(directory)
