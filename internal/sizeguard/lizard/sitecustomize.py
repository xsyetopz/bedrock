"""Apply bedrock's lizard patches as lizard imports the modules they change.

bedrock runs the pinned lizard release with this directory first on
PYTHONPATH, so Python imports this file at startup. Each `*.patch` beside it
is a unified diff against that release; a module a patch names is compiled
from its patched source in place of the installed one. A hunk that no longer
matches stops the import, since a measure taken by an unpatched module
would be silently wrong.
"""

import importlib.abc
import importlib.machinery
import importlib.util
import os
import re
import sys

_HERE = os.path.dirname(os.path.abspath(__file__))
_HUNK = re.compile(r"^@@ -(\d+)(?:,(\d+))? \+\d+(?:,(\d+))? @@")


def _fail(reason):
    """Stop the run, since a measure taken unpatched would be wrong."""
    # site.py turns an exception from sitecustomize into a warning or a
    # traceback, and lizard one from an import into a message of its own, so
    # the hook prints the reason and exits itself.
    print(reason, file=sys.stderr, flush=True)
    os._exit(2)


def _read_patches():
    """Map each patched module name to the hunks of its diffs, in order."""
    patches = {}
    for name in sorted(os.listdir(_HERE)):
        if not name.endswith(".patch"):
            continue
        with open(os.path.join(_HERE, name), encoding="utf-8") as patch:
            module, body, old, new = None, [], 0, 0
            for line in patch.read().splitlines():
                if old > 0 or new > 0:
                    # An editor may strip the space of an empty context line.
                    line = line or " "
                    body.append(line)
                    old -= line[0] in (" ", "-")
                    new -= line[0] in (" ", "+")
                elif line.startswith("+++ "):
                    target = line[4:].split("\t")[0]
                    target = target[2:] if target.startswith("b/") else target
                    module = target[:-len(".py")].replace("/", ".")
                    if module in patches:
                        # Two diffs against one release would each count
                        # lines from the unpatched file and collide.
                        _fail(f"bedrock: {name} patches {module}, which "
                              "another patch changes; keep one patch per "
                              "module")
                    patches[module] = []
                elif module and _HUNK.match(line):
                    header = _HUNK.match(line)
                    old = int(header.group(2) or 1)
                    new = int(header.group(3) or 1)
                    body = []
                    patches[module].append((name, int(header.group(1)), body))
    return patches


def _apply(source, hunks, module):
    """Return source with each hunk applied where its old lines stand."""
    lines = source.splitlines()
    shift = 0
    for name, start, body in hunks:
        old = [line[1:] for line in body if line[0] in (" ", "-")]
        new = [line[1:] for line in body if line[0] in (" ", "+")]
        at = start - 1 + shift
        if lines[at:at + len(old)] != old:
            _fail(f"bedrock: {name} no longer applies to {module}; "
                  "update the patch for the lizard release bedrock pins")
        lines[at:at + len(old)] = new
        shift += len(new) - len(old)
    return "\n".join(lines) + "\n"


class _PatchedLoader(importlib.machinery.SourceFileLoader):
    """Compile the patched source, never a cached bytecode file."""

    def __init__(self, fullname, path, hunks):
        super().__init__(fullname, path)
        self._hunks = hunks

    def get_code(self, fullname):
        with open(self.path, encoding="utf-8") as original:
            source = _apply(original.read(), self._hunks, fullname)
        return compile(source, self.path, "exec", dont_inherit=True)


class _PatchFinder(importlib.abc.MetaPathFinder):
    """Find a patched module where Python would, and load it patched."""

    def __init__(self, patches):
        self._patches = patches

    def find_spec(self, fullname, path, target=None):
        hunks = self._patches.get(fullname)
        if hunks is None:
            return None
        spec = importlib.machinery.PathFinder.find_spec(fullname, path)
        if spec is None or not spec.origin or not spec.origin.endswith(".py"):
            return spec
        return importlib.util.spec_from_file_location(
            fullname, spec.origin,
            loader=_PatchedLoader(fullname, spec.origin, hunks),
            submodule_search_locations=spec.submodule_search_locations)


def _chain():
    """Run the sitecustomize this one shadows, as some distributions ship."""
    others = [entry for entry in sys.path
              if os.path.abspath(entry or ".") != _HERE]
    spec = importlib.machinery.PathFinder.find_spec("sitecustomize", others)
    if spec and spec.loader:
        spec.loader.exec_module(importlib.util.module_from_spec(spec))


sys.meta_path.insert(0, _PatchFinder(_read_patches()))
_chain()
