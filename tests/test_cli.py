import subprocess
import sys
from pathlib import Path

import yaml

from awml import draw, page

ROOT = Path(__file__).resolve().parent.parent
GOOD = {"awml": "0.1", "title": "Dinners", "view": "scopes",
        "groups": [{"id": "family", "label": "family"}],
        "parts": [{"id": "thu", "kind": "trigger", "label": "Thursday 19:00"},
                  {"id": "plan", "kind": "ai", "label": "Plan next week's dinners", "in": "family"}],
        "lines": [{"from": "thu", "to": "plan", "kind": "start"}]}
BAD = dict(GOOD, lines=[{"from": "thu", "to": "plann", "kind": "start"}])


def awml(*args):
    return subprocess.run([sys.executable, "-m", "awml", *map(str, args)], cwd=ROOT, capture_output=True, text=True)


def write(tmp_path, name, d):
    p = tmp_path / name
    p.write_text(yaml.safe_dump(d, sort_keys=False))
    return p


def test_draw_prints_mermaid(tmp_path):
    r = awml("draw", write(tmp_path, "good.yaml", GOOD))
    assert r.returncode == 0
    assert r.stdout == draw(GOOD) + "\n"


def test_draw_refuses_a_file_with_mistakes(tmp_path):
    f = write(tmp_path, "bad.yaml", BAD)
    r = awml("draw", f)
    assert r.returncode == 1
    assert r.stdout == ""
    assert r.stderr == f'{f}: the line from "thu" to "plann" ends at unknown part "plann"\n'


def test_check_passes_a_sound_file(tmp_path):
    f = write(tmp_path, "good.yaml", GOOD)
    r = awml("check", f)
    assert (r.returncode, r.stdout) == (0, f"{f}: ok\n")


def test_check_reports_each_mistake_and_fails(tmp_path):
    good, bad = write(tmp_path, "good.yaml", GOOD), write(tmp_path, "bad.yaml", BAD)
    r = awml("check", good, bad)
    assert r.returncode == 1
    assert r.stdout == f'{good}: ok\n{bad}: the line from "thu" to "plann" ends at unknown part "plann"\n'


def test_check_reports_a_file_that_isnt_yaml(tmp_path):
    f = tmp_path / "broken.yaml"
    f.write_text("parts: [\n")
    r = awml("check", f)
    assert r.returncode == 1
    assert r.stdout.startswith(f"{f}: isn't YAML: ")


def test_check_reports_a_missing_file(tmp_path):
    f = tmp_path / "none.yaml"
    r = awml("check", f)
    assert r.returncode == 1
    assert r.stdout.startswith(f"{f}: can't be read: ")


def test_page_writes_one_html_file(tmp_path):
    a = write(tmp_path, "a.yaml", GOOD)
    b = write(tmp_path, "b.yaml", dict(GOOD, view="where"))
    out = tmp_path / "out.html"
    r = awml("page", a, b, "-o", out)
    assert (r.returncode, r.stdout, r.stderr) == (0, "", "")
    assert out.read_text() == page([GOOD, dict(GOOD, view="where")])


def test_page_writes_nothing_when_a_file_has_mistakes(tmp_path):
    out = tmp_path / "out.html"
    r = awml("page", write(tmp_path, "a.yaml", GOOD), write(tmp_path, "bad.yaml", BAD), "-o", out)
    assert r.returncode == 1
    assert "unknown part" in r.stderr
    assert not out.exists()
