#!/usr/bin/env python3
"""Rejects interface strings that break the house rules for Russian copy."""

import pathlib
import re
import sys

RULES = {
    "long dash": re.compile(r"[—–]"),
    "colon": re.compile(r":"),
    "plus sign": re.compile(r"\+"),
    "comparative": re.compile(r"\b(быстрее|лучше|проще|надёжнее|удобнее|дешевле)\b", re.I),
    "transliteration": re.compile(r"\b(хеш\w*|бэкап\w*|чекбокс\w*|верификац\w*|валидац\w*)\b", re.I),
    "first person": re.compile(r"\b(считаем|сверяем|передаём|раскладываем|перечитываем|трогаем)\b", re.I),
}

PATTERNS = [
    re.compile(r'"([^"\n]*[А-Яа-яЁё][^"\n]*)"'),
    re.compile(r">\s*([^<>{}\n][^<>{}]*[А-Яа-яЁё][^<>{}]*?)\s*<"),
    re.compile(r"`([^`\n]*[А-Яа-яЁё][^`\n]*)`"),
]

root = pathlib.Path(__file__).resolve().parent.parent / "web" / "src"
checked = failed = 0

for path in sorted(root.rglob("*.ts*")):
    text = path.read_text(encoding="utf-8")
    for pattern in PATTERNS:
        for match in pattern.finditer(text):
            phrase = match.group(1).strip()
            if not phrase:
                continue
            prose = re.sub(r"\w+://\S*", "", phrase)
            checked += 1
            for name, rule in RULES.items():
                if rule.search(prose):
                    line = text[: match.start()].count("\n") + 1
                    print(f"{path.name}:{line} [{name}] {phrase[:90]}")
                    failed += 1

print(f"checked {checked} strings, {failed} rejected")
sys.exit(1 if failed else 0)
