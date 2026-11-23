"""Winnowing on Java tokens with original Unicode character positions."""

from __future__ import annotations

import hashlib
import re
import unicodedata
from collections import defaultdict, deque
from dataclasses import dataclass
from typing import Protocol

KEYWORDS = {
    "abstract",
    "assert",
    "boolean",
    "break",
    "byte",
    "case",
    "catch",
    "char",
    "class",
    "const",
    "continue",
    "default",
    "do",
    "double",
    "else",
    "enum",
    "extends",
    "final",
    "finally",
    "float",
    "for",
    "goto",
    "if",
    "implements",
    "import",
    "instanceof",
    "int",
    "interface",
    "long",
    "native",
    "new",
    "package",
    "private",
    "protected",
    "public",
    "return",
    "short",
    "static",
    "strictfp",
    "super",
    "switch",
    "synchronized",
    "this",
    "throw",
    "throws",
    "transient",
    "try",
    "void",
    "volatile",
    "while",
    "_",
}
OPERATORS = re.compile(
    r">>>=|>>=|<<=|>>>|\.\.\.|>>|<<|::|->|\+\+|--|&&|\|\||[+\-*/%&|^!<>=]="
)
DIGITS = r"[0-9](?:_*[0-9])*"
HEX = r"[0-9a-fA-F](?:_*[0-9a-fA-F])*"
NUMBER = re.compile(
    rf"0[xX](?:{HEX}(?:\.(?:{HEX})?)?|\.{HEX})[pP][+-]?{DIGITS}[fFdD]?"
    rf"|0[xX]{HEX}[lL]?|0[bB][01](?:_*[01])*[lL]?"
    rf"|(?:{DIGITS}\.(?:{DIGITS})?|\.{DIGITS})(?:[eE][+-]?{DIGITS})?[fFdD]?"
    rf"|{DIGITS}[eE][+-]?{DIGITS}[fFdD]?|{DIGITS}[fFdDlL]?"
)


@dataclass(frozen=True, slots=True)
class Token:
    raw: str
    value: str
    start: int
    end: int


class Lexer(Protocol):
    def tokenize(self, text: str) -> list[Token]: ...


def _identifier_start(c: str) -> bool:
    return c.isalpha() or c in "_$" or unicodedata.category(c) in {"Nl", "Sc", "Pc"}


def _identifier_part(c: str) -> bool:
    return _identifier_start(c) or unicodedata.category(c) in {"Nd", "Mn", "Mc", "Cf"}


def _literal_end(text: str, start: int, delimiter: str) -> int:
    index = start + len(delimiter)
    while index < len(text):
        if text.startswith(delimiter, index):
            return index + len(delimiter)
        if text[index] in "\r\n" and len(delimiter) == 1:
            break
        index += 2 if text[index] == "\\" else 1
    raise ValueError(f"Unterminated Java literal at character {start}")


class JavaLexer:
    def tokenize(self, text: str) -> list[Token]:
        out = []
        index = 0
        while index < len(text):
            start = index
            c = text[index]
            if c.isspace():
                index += 1
                continue
            if text.startswith("//", index):
                while index < len(text) and text[index] not in "\r\n":
                    index += 1
                continue
            if text.startswith("/*", index):
                end = text.find("*/", index + 2)
                if end < 0:
                    raise ValueError(f"Unterminated Java comment at character {index}")
                index = end + 2
                continue
            if c in "\"'":
                delim = '"""' if text.startswith('"""', index) else c
                index = _literal_end(text, index, delim)
                value = "@char" if delim == "'" else "@string"
            elif _identifier_start(c):
                index += 1
                while index < len(text) and _identifier_part(text[index]):
                    index += 1
                raw = text[start:index]
                if raw in {"true", "false"}:
                    value = "@boolean"
                elif raw == "null" or raw in KEYWORDS:
                    value = raw
                else:
                    value = "@identifier"
            elif (number := NUMBER.match(text, index)) is not None:
                index = number.end()
                value = "@number"
            else:
                operator = OPERATORS.match(text, index)
                index = operator.end() if operator else index + 1
                value = text[start:index]
            out.append(Token(text[start:index], value, start, index))
        return out


Gram = tuple[str, ...]


def _hash(gram: Gram) -> int:
    # Individual normalized Java token values contain no NUL separator.
    return int.from_bytes(
        hashlib.blake2b("\0".join(gram).encode(), digest_size=8).digest(), "big"
    )


def winnow(hashes: list[int], w: int) -> list[int]:
    """Select the rightmost minimum per window, emitting each position once."""
    if w < 1:
        raise ValueError("w must be positive")
    width = min(w, len(hashes))
    minima: deque[int] = deque()
    selected = []
    for index, value in enumerate(hashes):
        while minima and hashes[minima[-1]] >= value:
            minima.pop()
        minima.append(index)
        if minima[0] <= index - width:
            minima.popleft()
        if index + 1 >= width and (not selected or selected[-1] != minima[0]):
            selected.append(minima[0])
    return selected


@dataclass
class Window:
    positions: dict[Gram, list[int]]


@dataclass
class Prepared:
    tokens: list[Token]
    windows: list[Window]
    k: int
    w: int


def template_grams(text: str, *, k: int = 5, lexer: Lexer | None = None) -> set[Gram]:
    if k < 1:
        raise ValueError("k must be positive")
    values = [token.value for token in (lexer or JavaLexer()).tokenize(text)]
    return {tuple(values[i : i + k]) for i in range(len(values) - k + 1)}


def prepare(
    text: str,
    *,
    k: int = 5,
    w: int = 4,
    lexer: Lexer | None = None,
    excluded: set[Gram] | None = None,
) -> Prepared:
    if not 1 <= k <= 200 or w < 1:
        raise ValueError("Expected 1 <= k <= 200 and w >= 1")
    tokens = (lexer or JavaLexer()).tokenize(text)
    values = [token.value for token in tokens]
    grams = [tuple(values[i : i + k]) for i in range(len(values) - k + 1)]
    selected = winnow([_hash(gram) for gram in grams], w)
    last = max(0, len(tokens) - 200)
    starts = list(range(0, last, 100)) + [last]
    windows = []
    offset = 0
    for start in starts:
        while offset < len(selected) and selected[offset] < start:
            offset += 1
        positions = defaultdict(list)
        j = offset
        while j < len(selected) and selected[j] + k <= start + 200:
            position = selected[j]
            if not excluded or grams[position] not in excluded:
                positions[grams[position]].append(position)
            j += 1
        if positions:
            windows.append(Window(dict(positions)))
    return Prepared(tokens, windows, k, w)


def _winner(a: Prepared, b: Prepared) -> tuple[float, Window | None, Window | None]:
    if (a.k, a.w) != (b.k, b.w):
        raise ValueError("Incompatible fingerprint parameters")
    best = 0.0
    left = right = None
    for x in a.windows:
        for y in b.windows:
            common = len(x.positions.keys() & y.positions.keys())
            score = common / (len(x.positions) + len(y.positions) - common)
            if score > best:
                best, left, right = score, x, y
    return best, left, right


def similarity(a: Prepared, b: Prepared) -> float:
    """Maximum verified local Jaccard over distinct selected token k-grams."""
    return _winner(a, b)[0]


def exact_token_match(a: Prepared, b: Prepared) -> bool:
    """Comparator: identical complete raw token sequences after skipping trivia."""
    return (
        bool(a.windows and b.windows)
        and len(a.tokens) == len(b.tokens)
        and all(x.raw == y.raw for x, y in zip(a.tokens, b.tokens, strict=True))
    )


def overlap_spans(a: Prepared, b: Prepared) -> list[dict]:
    """Merge verified anchors on each constant token offset in the best window pair."""
    _, left, right = _winner(a, b)
    if left is None or right is None:
        return []
    diagonals: dict[int, list[int]] = defaultdict(list)
    for gram in left.positions.keys() & right.positions.keys():
        for p in left.positions[gram]:
            for q in right.positions[gram]:
                diagonals[q - p].append(p)
    spans = []
    for delta, starts in diagonals.items():
        runs: list[list[int]] = []
        for start in sorted(starts):
            if runs and start <= runs[-1][1]:
                runs[-1][1] = max(runs[-1][1], start + a.k)
            else:
                runs.append([start, start + a.k])
        for start, end in runs:
            spans.append(
                {
                    "target": [a.tokens[start].start, a.tokens[end - 1].end],
                    "source": [
                        b.tokens[start + delta].start,
                        b.tokens[end - 1 + delta].end,
                    ],
                    "token_count": end - start,
                }
            )
    return sorted(spans, key=lambda span: (span["target"], span["source"]))
