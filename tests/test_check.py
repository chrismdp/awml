import copy

import pytest

from awml import check

GOOD = {
    "awml": "0.1", "title": "The family's dinners", "view": "scopes",
    "groups": [{"id": "carers", "label": "carers"}, {"id": "family", "label": "family", "in": "carers"}],
    "parts": [{"id": "thu", "kind": "trigger", "label": "Thursday 19:00"},
              {"id": "plan", "kind": "ai", "label": "Plan next week's dinners", "in": "family"},
              {"id": "mealplan", "kind": "context", "label": "Meal plan", "in": "family"},
              {"id": "split", "kind": "fork", "in": "family"},
              {"id": "note", "kind": "comment", "label": "For example: scored"}],
    "lines": [{"from": "thu", "to": "plan", "kind": "start"},
              {"from": "plan", "to": "mealplan", "kind": "data", "label": "posted", "breaks": True},
              {"from": "mealplan", "to": "split", "kind": "data"},
              {"from": "note", "to": "plan", "kind": "tie"},
              {"from": "note", "to": "thu", "kind": "hidden"}],
}


def changed(**edits):
    d = copy.deepcopy(GOOD)
    for path, value in edits.items():
        *keys, last = path.split("__")
        target = d
        for k in keys:
            target = target[int(k)] if isinstance(target, list) else target[k]
        if value is None:
            del target[int(last) if isinstance(target, list) else last]
        else:
            target[int(last) if isinstance(target, list) else last] = value
    return d


def test_a_sound_file_has_no_mistakes():
    assert check(GOOD) == []


def test_an_unknown_part_kind():
    assert check(changed(parts__1__kind="agent")) == ['part "plan" has unknown kind "agent"']


def test_an_unknown_line_kind():
    assert check(changed(lines__0__kind="dotted")) == ['the line from "thu" to "plan" has unknown kind "dotted"']


def test_an_unknown_view():
    assert check(changed(view="who")) == ['view must be "scopes" or "where", not "who"']


def test_an_unknown_version():
    assert check(changed(awml="2")) == ['awml must be "0.1", not "2"']


def test_a_part_in_an_unknown_group():
    assert check(changed(parts__1__in="house")) == ['part "plan" is in unknown group "house"']


def test_a_group_in_an_unknown_group():
    assert check(changed(groups__1__in="house")) == ['group "family" is in unknown group "house"']


def test_a_line_to_an_unknown_part():
    assert check(changed(lines__1__to="plann")) == ['the line from "plan" to "plann" ends at unknown part "plann"']


def test_a_line_from_an_unknown_part():
    assert check(changed(lines__0__from="fri")) == ['the line from "fri" to "plan" starts at unknown part "fri"']


def test_a_line_to_a_group_is_to_an_unknown_part():
    assert check(changed(lines__0__to="family")) == ['the line from "thu" to "family" ends at unknown part "family"']


@pytest.mark.parametrize("kind", ["start", "data", "publish", "refused"])
def test_a_comment_linked_by_anything_but_tie(kind):
    assert check(changed(lines__3__kind=kind)) == [f'comment "note" is linked by a {kind} line: a comment takes only tie']


def test_a_tie_with_no_comment():
    assert check(changed(lines__1__kind="tie")) == ['the tie from "plan" to "mealplan" has no comment at either end']


def test_a_group_nested_in_itself():
    assert check(changed(groups__0__in="carers")) == ['group "carers" is nested in itself']


def test_groups_nested_in_each_other():
    assert check(changed(groups__0__in="family")) == ['group "carers" is nested in itself',
                                                      'group "family" is nested in itself']


def test_an_id_used_twice():
    assert check(changed(parts__2__id="plan", lines__1__to="plan", lines__2__from="plan")) == ['id "plan" is used twice']


def test_an_id_mermaid_cant_read():
    d = changed(parts__1__id="plan-it", lines__0__to="plan-it", lines__1__from="plan-it", lines__3__to="plan-it")
    assert check(d) == ['id "plan-it" must be letters, digits and underscores, starting with a letter']


def test_a_missing_field():
    assert check(changed(parts__1__label=None)) == ['part "plan" has no label']
    assert check(changed(lines__0__kind=None)) == ['the line from "thu" to "plan" has no kind']
    assert check(changed(title=None)) == ["the file has no title"]


def test_the_version_may_be_written_as_a_number():
    assert check(changed(awml=0.1)) == []


def test_an_unknown_field():
    assert check(changed(lines__1__brakes=True)) == ['the line from "plan" to "mealplan" has unknown field "brakes"']
    assert check(changed(colour="red")) == ['the file has unknown field "colour"']


def test_not_a_file_of_parts():
    assert check(["a", "b"]) == ["the file must be a mapping of fields, starting with awml"]
    assert check(changed(parts="none")) == ["parts must be a list"]
    assert check(changed(parts__0="thu")) == ["each part must be a mapping of fields"]


def test_every_mistake_is_reported_at_once():
    assert check(changed(awml="2", lines__1__to="plann")) == ['awml must be "0.1", not "2"',
                                                             'the line from "plan" to "plann" ends at unknown part "plann"']
