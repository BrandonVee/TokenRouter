#!/usr/bin/env python3
"""检查 Git 跟踪文件中的常见凭据，诊断只输出位置和类型，不回显秘密。"""

from __future__ import annotations

import re
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path


# 只匹配提供商的明确凭据形状，不用泛化的 password/token 字段误伤配置与合法示例。
RULES = {
    "private_key": re.compile(rb"-----BEGIN (?:RSA |EC |DSA |OPENSSH |ENCRYPTED )?PRIVATE KEY-----(?:\r?\n|\\n)(?:[A-Za-z0-9+/=]{16,}(?:\r?\n|\\n)){2,}-----END (?:RSA |EC |DSA |OPENSSH |ENCRYPTED )?PRIVATE KEY-----"),
    "github_token": re.compile(rb"(?<![A-Za-z0-9_])(?:gh[pousr]_[A-Za-z0-9]{36,}|github_pat_[A-Za-z0-9_]{60,})(?![A-Za-z0-9_])"),
    "aws_access_key": re.compile(rb"(?<![A-Za-z0-9])(?:AKIA|ASIA)[A-Z0-9]{16}(?![A-Za-z0-9])"),
    "google_api_key": re.compile(rb"(?<![A-Za-z0-9_])AIza[A-Za-z0-9_-]{35}(?![A-Za-z0-9_-])"),
    "google_oauth_secret": re.compile(rb"(?<![A-Za-z0-9_])GOCSPX-[A-Za-z0-9_-]{28,}(?![A-Za-z0-9_-])"),
    "openai_api_key": re.compile(rb"(?<![A-Za-z0-9_-])sk-(?:proj-|svcacct-)?[A-Za-z0-9_-]{32,}(?![A-Za-z0-9_-])"),
    "slack_token": re.compile(rb"(?<![A-Za-z0-9_])xox[baprs]-[A-Za-z0-9-]{24,}(?![A-Za-z0-9-])"),
}
AWS_DOCUMENTATION_EXAMPLE = b"AKIA" + b"IOSFODNN7EXAMPLE"
ALLOW_MARKER = b"secret-scan: allow"


@dataclass(frozen=True)
class Finding:
    path: str
    line: int
    kind: str


def is_placeholder(value: bytes) -> bool:
    """排除明确的示例前缀、公开 AWS 示例与重复字符夹具，不全局豁免测试目录。"""
    if value == AWS_DOCUMENTATION_EXAMPLE:
        return True
    tail = re.sub(rb"^(?:sk-(?:proj-|svcacct-)?|gh[pousr]_|github_pat_|AIza|GOCSPX-)", b"", value)
    lower = tail.lower()
    return lower.startswith((b"test-", b"example-", b"placeholder-", b"your-")) or len(set(tail)) <= 3


def scan_bytes(path: str, data: bytes) -> list[Finding]:
    """按字节扫描，保证非法编码也能被检查；二进制资源不作为文本解析。"""
    if b"\0" in data[:8192]:
        return []
    findings: list[Finding] = []
    # PEM 要求完整的编码内容，只有 BEGIN 标记的解析失败夹具不属于泄露。
    for match in RULES["private_key"].finditer(data):
        number = data[:match.start()].count(b"\n") + 1
        if ALLOW_MARKER not in data.splitlines()[number - 1]:
            findings.append(Finding(path, number, "private_key"))
    for number, line in enumerate(data.splitlines(), 1):
        if ALLOW_MARKER in line:
            continue
        for kind, rule in RULES.items():
            if kind == "private_key":
                continue
            if any(not is_placeholder(match.group()) for match in rule.finditer(line)):
                findings.append(Finding(path, number, kind))
    return findings


def tracked_paths(root: Path) -> list[Path]:
    """使用 NUL 分隔支持空格等文件名；不读取未跟踪的环境文件或符号链接目标。"""
    output = subprocess.check_output(["git", "-C", str(root), "ls-files", "-z"])
    return [Path(part.decode("utf-8", errors="surrogateescape")) for part in output.split(b"\0") if part]


def main() -> int:
    try:
        root = Path(subprocess.check_output(["git", "rev-parse", "--show-toplevel"], text=True).strip())
        findings: list[Finding] = []
        failures: list[str] = []
        for relative in tracked_paths(root):
            path = root / relative
            if path.is_symlink() or not path.exists():
                continue
            try:
                findings.extend(scan_bytes(str(relative), path.read_bytes()))
            except OSError:
                failures.append(str(relative))
    except (OSError, subprocess.CalledProcessError):
        print("凭据扫描失败：无法读取 Git 跟踪文件。", file=sys.stderr)
        return 2
    for finding in findings:
        print(f"{finding.path}:{finding.line}: {finding.kind}")
    for path in failures:
        print(f"{path}: 无法读取，扫描不完整。", file=sys.stderr)
    if failures:
        return 2
    if findings:
        print(f"发现 {len(findings)} 处疑似凭据；确认是公开夹具时可在该行标注 secret-scan: allow。", file=sys.stderr)
        return 1
    print("凭据扫描通过。")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
