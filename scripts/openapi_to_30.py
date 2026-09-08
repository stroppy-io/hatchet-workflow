#!/usr/bin/env python3
"""Project the 3.1 source spec down to an OpenAPI 3.0 document for ogen.

ogen (v1.20.x) parses OpenAPI 3.0 only: a single `type` string plus
`nullable: true`. Our source of truth (openapi/openapi.yaml) is strict 3.1,
where nullability is expressed as `type: [T, "null"]` (since `nullable` is
invalid in 3.1). This script performs the lossless-for-codegen down-projection:

  - openapi 3.1.0            -> 3.0.3
  - type: [T, "null"]        -> type: T   + nullable: true
  - type: [T]                -> type: T
  - anyOf: [X, {type: null}] -> allOf: [X] + nullable: true
  - enum: [..., null]        -> enum without null (+ nullable already set)
  - drop info.summary        (3.1-only field)

It is invoked from the Makefile (`make generate-go`); the output is a build
artifact under openapi/.build/ and is not committed.
"""
import sys
import copy

try:
    import yaml
except ImportError:
    sys.exit("PyYAML required: pip install pyyaml")


def convert(node):
    if isinstance(node, dict):
        # anyOf: [X, {type: null}] -> allOf: [X] + nullable
        anyof = node.get("anyOf")
        if isinstance(anyof, list):
            non_null = [b for b in anyof if not (isinstance(b, dict) and b.get("type") == "null")]
            if len(non_null) != len(anyof):
                node.pop("anyOf")
                if len(non_null) == 1:
                    node["allOf"] = [non_null[0]]
                else:
                    node["anyOf"] = non_null
                node["nullable"] = True

        # type: [T, "null"] / [T] -> single type (+ nullable)
        t = node.get("type")
        if isinstance(t, list):
            has_null = "null" in t
            concrete = [x for x in t if x != "null"]
            if len(concrete) > 1:
                sys.exit(f"cannot down-convert multi-type {t!r} (no 3.0 equivalent)")
            if concrete:
                node["type"] = concrete[0]
            else:
                node.pop("type")
            if has_null:
                node["nullable"] = True

        # enum containing null -> drop null, mark nullable
        en = node.get("enum")
        if isinstance(en, list) and None in en:
            node["enum"] = [x for x in en if x is not None]
            node["nullable"] = True

        for v in node.values():
            convert(v)
    elif isinstance(node, list):
        for v in node:
            convert(v)
    return node


def main():
    if len(sys.argv) != 3:
        sys.exit("usage: openapi_to_30.py <src-3.1.yaml> <dst-3.0.yaml>")
    src, dst = sys.argv[1], sys.argv[2]
    spec = yaml.safe_load(open(src))
    spec["openapi"] = "3.0.3"
    spec.get("info", {}).pop("summary", None)
    convert(spec)
    with open(dst, "w") as f:
        yaml.safe_dump(spec, f, sort_keys=False, allow_unicode=True, width=100)
    print(f"wrote {dst} (OpenAPI 3.0.3 projection)")


if __name__ == "__main__":
    main()
