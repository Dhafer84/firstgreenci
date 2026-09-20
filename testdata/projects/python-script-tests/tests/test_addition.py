"""Test écrit pour être lancé directement, sans pytest ni unittest.TestCase.

    python3 -B tests/test_addition.py
"""


def test_addition():
    assert 1 + 1 == 2


def main() -> int:
    test_addition()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
