"""The awml command: draw, page and check."""
import argparse
import sys

import yaml

from . import check, draw, page


def read(path):
    """The document in a file, and the mistakes that stop it being drawn."""
    try:
        with open(path) as f:
            doc = yaml.safe_load(f)
    except OSError as e:
        return None, [f"can't be read: {e.strerror}"]
    except yaml.YAMLError as e:
        return None, [f"isn't YAML: {' '.join(str(e).split())}"]
    return doc, check(doc)


def sound(paths):
    """Every file's document, or None once any file has mistakes, which are printed."""
    docs, ok = [], True
    for path in paths:
        doc, errors = read(path)
        for e in errors:
            print(f"{path}: {e}", file=sys.stderr)
        ok = ok and not errors
        docs.append(doc)
    return docs if ok else None


def main(argv=None):
    parser = argparse.ArgumentParser(prog="awml", description="Draw agentic workflows written in AWML.")
    sub = parser.add_subparsers(dest="command", required=True)
    sub.add_parser("draw", help="print a file's Mermaid").add_argument("file")
    p = sub.add_parser("page", help="write one HTML page that draws each file")
    p.add_argument("files", nargs="+")
    p.add_argument("-o", "--out", required=True)
    sub.add_parser("check", help="report a file's structural mistakes").add_argument("files", nargs="+")
    args = parser.parse_args(argv)

    if args.command == "check":
        failed = False
        for path in args.files:
            errors = read(path)[1]
            print("\n".join(f"{path}: {e}" for e in errors) if errors else f"{path}: ok")
            failed = failed or bool(errors)
        return 1 if failed else 0
    docs = sound([args.file] if args.command == "draw" else args.files)
    if docs is None:
        return 1
    if args.command == "draw":
        print(draw(docs[0]))
    else:
        with open(args.out, "w") as f:
            f.write(page(docs))
    return 0


if __name__ == "__main__":
    sys.exit(main())
