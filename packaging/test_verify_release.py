import hashlib
import pathlib
import subprocess
import sys
import tempfile
import unittest

SCRIPT = pathlib.Path(__file__).with_name("verify-release.py")


class ReleaseGateTests(unittest.TestCase):
    def test_requires_all_platforms_and_correct_checksums(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            for name in ["FreeTurn-Linux-amd64-v3.0.0.tar.gz", "FreeTurn-Windows-amd64-v3.0.0.zip",
                         "FreeTurn-macOS-arm64-v3.0.0.zip", "FreeTurn-macOS-amd64-v3.0.0.zip"]:
                data = name.encode()
                (root / name).write_bytes(data)
                (root / (name + ".sha256")).write_text(f"{hashlib.sha256(data).hexdigest()}  {name}\n")
            def check():
                return subprocess.run([sys.executable, str(SCRIPT), str(root), "v3.0.0"], capture_output=True).returncode
            self.assertEqual(check(), 0)
            archive = root / "FreeTurn-Windows-amd64-v3.0.0.zip"
            original = archive.read_bytes()
            archive.write_bytes(b"tampered")
            self.assertNotEqual(check(), 0)
            archive.write_bytes(original)
            archive.unlink()
            self.assertNotEqual(check(), 0)


if __name__ == "__main__":
    unittest.main()
