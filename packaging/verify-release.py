"""Refuse to publish an incomplete or incorrectly checksummed desktop release."""
import hashlib
import pathlib
import re
import sys

root = pathlib.Path(sys.argv[1])
tag = sys.argv[2]
if not re.fullmatch(r"v\d+\.\d+\.\d+", tag):
    raise SystemExit("Invalid release tag")
expected = {
    f"FreeTurn-Linux-amd64-{tag}.tar.gz",
    f"FreeTurn-Windows-amd64-{tag}.zip",
    f"FreeTurn-macOS-arm64-{tag}.zip",
    f"FreeTurn-macOS-amd64-{tag}.zip",
}
actual = {p.name for p in root.iterdir() if p.suffix in {".gz", ".zip"}}
if actual != expected:
    raise SystemExit(f"Wrong release archives: missing={expected-actual}, extra={actual-expected}")
for name in sorted(expected):
    archive = root / name
    if not archive.stat().st_size:
        raise SystemExit(f"Empty archive: {name}")
    digest = hashlib.file_digest(archive.open("rb"), "sha256").hexdigest()
    if (root / (name + ".sha256")).read_text().strip() != f"{digest}  {name}":
        raise SystemExit(f"Checksum mismatch: {name}")
print("Verified all four native desktop archives")
