from pathlib import Path

import pytest
import yaml

from awml import check, draw

EXAMPLES = sorted((Path(__file__).resolve().parent.parent / "examples").glob("*.yaml"))


def test_there_are_examples():
    assert [p.stem for p in EXAMPLES] == ["client-email-where", "client-email", "dinners-scopes", "dinners-where", "question"]


@pytest.mark.parametrize("path", EXAMPLES, ids=lambda p: p.stem)
def test_each_example_is_sound_and_its_drawings_are_current(path):
    doc = yaml.safe_load(path.read_text())
    assert check(doc) == []
    assert path.with_suffix(".mmd").read_text() == draw(doc) + "\n"
    assert path.with_suffix(".svg").read_text().startswith("<svg")
