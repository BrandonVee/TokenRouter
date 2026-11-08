"""验证扫描器检出凭据、位置诊断和公开夹具豁免，不把测试凭据写入源码。"""

import unittest

from secret_scan import scan_bytes


class SecretScanTest(unittest.TestCase):
    def test_provider_keys_are_detected_without_leaking_values(self):
        value = b"ghp_" + b"a1b2c3d4e5f6" * 3
        result = scan_bytes("a file.txt", b"safe\n" + value + b"\n")
        self.assertEqual([(item.path, item.line, item.kind) for item in result], [("a file.txt", 2, "github_token")])
        self.assertNotIn(value.decode(), repr(result))

    def test_test_directory_is_not_globally_exempted(self):
        self.assertTrue(scan_bytes("tests/fixture.txt", b"sk-proj-" + b"Ab2Cd3Ef4Gh5" * 4))

    def test_placeholder_and_explicit_line_exemption(self):
        self.assertFalse(scan_bytes("example.txt", b"sk-" + b"x" * 48))
        self.assertFalse(scan_bytes("example.txt", b"ghp_" + b"a1b2c3d4e5f6" * 3 + b" # secret-scan: allow"))

    def test_binary_file_and_invalid_encoding(self):
        value = b"GOCSPX-" + b"a1b2c3d4" * 4
        self.assertFalse(scan_bytes("image.bin", b"\0" + value))
        self.assertTrue(scan_bytes("text.txt", b"\xff" + value))

    def test_pem_header_is_detected(self):
        fixture = b"-----BEGIN " + b"RSA PRIVATE KEY-----\n" + b"QUJDREVGR0hJSktM\n" * 2 + b"-----END " + b"RSA PRIVATE KEY-----"
        self.assertTrue(scan_bytes("key.pem", fixture))
        self.assertFalse(scan_bytes("invalid.txt", b"-----BEGIN " + b"RSA PRIVATE KEY-----"))


if __name__ == "__main__":
    unittest.main()
