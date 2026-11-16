#!/usr/bin/env python3
"""The paper as Markdown: build/md/paper.md and its figures in build/md/figures/.

    make md                      (builds main.pdf first, then runs this)
    python3 to-markdown.py       (from paper/, with main.aux and main.bbl current)

pandoc alone cannot read main.tex: its numbers are \\result{key}, looked up in
tables/numbers.tex through \\csname; its tables are \\input from tables/; its
figures are pgfplots. So the LaTeX is resolved here first, the way TeX would:

  - \\result{key} and \\result{laptop.key} become their values, \\resulttable and
    \\laptoptable the tables they read, \\IfFileExists its branch, and the
    macros the sections define (\\vs, \\vscell, ...) their expansion;
  - each figure is compiled on its own (tectonic, the preview package, with the
    paper's preamble), rendered to PNG and put in the text as an image;
  - \\ref and \\cite become the numbers LaTeX gave them, read from main.aux, and
    the bibliography is main.bbl, so the Markdown numbers what the PDF numbers;
  - sections are numbered as in the PDF, and captions read "Figure N:".

What is left is plain LaTeX, which pandoc turns into GitHub-flavoured Markdown.
"""

import argparse
import os
import re
import shutil
import subprocess
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))


def die(msg):
    sys.exit(f"to-markdown: {msg}")


def warn(msg):
    print(f"to-markdown: {msg}", file=sys.stderr)


def read(path):
    with open(path, encoding="utf-8") as f:
        return f.read()


# ---- TeX scanning ----------------------------------------------------------


def skip_space(s, i):
    while i < len(s) and s[i] in " \t\n":
        i += 1
    return i


def group(s, i):
    """The {...} at s[i] (after spaces): its content and the index past it."""
    i = skip_space(s, i)
    if i >= len(s) or s[i] != "{":
        raise ValueError(f"expected {{ at: {s[i:i + 60]!r}")
    depth, j = 0, i
    while j < len(s):
        c = s[j]
        if c == "\\":
            j += 2
            continue
        if c == "{":
            depth += 1
        elif c == "}":
            depth -= 1
            if depth == 0:
                return s[i + 1:j], j + 1
        j += 1
    raise ValueError(f"unbalanced group at: {s[i:i + 60]!r}")


def optional(s, i):
    """The [...] at s[i] (after spaces), if there is one."""
    k = skip_space(s, i)
    if k < len(s) and s[k] == "[":
        end = s.index("]", k)
        return s[k + 1:end], end + 1
    return None, i


def find_cmd(s, name, start=0):
    """Index of the next \\name that is not the start of a longer name."""
    pat = re.compile(r"\\" + re.escape(name) + r"(?![A-Za-z@])")
    m = pat.search(s, start)
    return m.start() if m else -1


def replace_cmd(s, name, nargs, fn, star=False):
    """Replace every \\name{a1}...{an} with fn(args) (fn(args, starred) if star)."""
    out, pos = [], 0
    while True:
        i = find_cmd(s, name, pos)
        if i < 0:
            break
        j = i + len(name) + 1
        starred = False
        if star and j < len(s) and s[j] == "*":
            starred, j = True, j + 1
        args = []
        for _ in range(nargs):
            a, j = group(s, j)
            args.append(a)
        out.append(s[pos:i])
        out.append(fn(args, starred) if star else fn(args))
        pos = j
    out.append(s[pos:])
    return "".join(out)


def strip_comments(s):
    """Drop % comments; a % at the end of a line also eats the newline, as in TeX."""
    s = re.sub(r"(?<!\\)%[^\n]*\n[ \t]*", "", s)
    return re.sub(r"(?<!\\)%[^\n]*$", "", s)


def environments(s, env):
    """(start, end, body) of every \\begin{env}...\\end{env}, outermost only."""
    begin, end = "\\begin{" + env + "}", "\\end{" + env + "}"
    found, pos = [], 0
    while True:
        i = s.find(begin, pos)
        if i < 0:
            return found
        j = s.find(end, i)
        if j < 0:
            die(f"\\begin{{{env}}} without \\end")
        found.append((i, j + len(end), s[i + len(begin):j]))
        pos = j + len(end)


# ---- what LaTeX already computed --------------------------------------------


def resultdefs(path, prefix=""):
    defs = {}
    if not os.path.exists(path):
        warn(f"{path} is not there: its results print as [key?]")
        return defs
    s = strip_comments(read(path))
    i = 0
    while True:
        i = find_cmd(s, "resultdef", i)
        if i < 0:
            return defs
        key, j = group(s, i + len("\\resultdef"))
        val, j = group(s, j)
        defs[prefix + key] = val
        i = j


def aux_refs(path):
    """label -> number, and bib key -> number, from main.aux."""
    s = read(path)
    labels, cites = {}, {}
    for m in re.finditer(r"\\newlabel\{([^}]*)\}\{\{([^}]*)\}", s):
        labels[m.group(1)] = m.group(2)
    for m in re.finditer(r"\\bibcite\{([^}]*)\}\{([^}]*)\}", s):
        cites[m.group(1)] = m.group(2)
    return labels, cites


def bibliography(path, cites):
    """main.bbl as numbered paragraphs, numbered as LaTeX numbered them."""
    s = strip_comments(read(path))
    body = s[s.index("\\begin{thebibliography}"):s.index("\\end{thebibliography}")]
    body = body[body.index("}", body.index("}") + 1) + 1:]  # past {thebibliography}{10}
    items = []
    for chunk in re.split(r"\\bibitem", body)[1:]:
        _, j = optional(chunk, 0)
        key, j = group(chunk, j)
        text = chunk[j:].replace("\\newblock", " ").strip()
        # {[}: a bare [ after a command would be read as its optional argument.
        items.append(f"{{[}}{cites.get(key, '?')}{{]}} {text}\n\n")
    return "\\section*{References}\n\n" + "".join(items)


# ---- the transformation -----------------------------------------------------


def inline_inputs(s):
    def one(args):
        name = args[0] if args[0].endswith(".tex") else args[0] + ".tex"
        return strip_comments(read(os.path.join(HERE, name)))
    return replace_cmd(s, "input", 1, one)


def collect_newcommands(s):
    """Remove the \\newcommand definitions from s; return (s, {name: (n, body)}, source)."""
    macros, sources = {}, []
    out, pos = [], 0
    while True:
        i = find_cmd(s, "newcommand", pos)
        if i < 0:
            break
        name, j = group(s, i + len("\\newcommand"))
        n, j = optional(s, j)
        body, j = group(s, j)
        macros[name.strip().lstrip("\\")] = (int(n or 0), body)
        sources.append(s[i:j])
        out.append(s[pos:i])
        pos = j
    out.append(s[pos:])
    return "".join(out), macros, "\n".join(sources)


def expand_macros(s, macros):
    for _ in range(10):  # macros that use macros
        before = s
        for name, (n, body) in macros.items():
            def one(args, body=body):
                r = body
                for k, a in enumerate(args, 1):
                    r = r.replace(f"#{k}", a)
                return r
            s = replace_cmd(s, name, n, one)
        if s == before:
            return s
    die("macros still expanding after 10 rounds")


def if_file_exists(s):
    def one(args):
        path, yes, no = args
        if "#" in path:  # inside a macro's body: left to TeX
            return None
        path = path.replace("\\tables", "tables").strip()
        return yes if os.path.exists(os.path.join(HERE, path)) else no
    out, pos = [], 0
    while True:
        i = find_cmd(s, "IfFileExists", pos)
        if i < 0:
            break
        j, args = i + len("\\IfFileExists"), []
        for _ in range(3):
            a, j = group(s, j)
            args.append(a)
        r = one(args)
        out.append(s[pos:i])
        out.append(s[i:j] if r is None else if_file_exists(r))
        pos = j
    out.append(s[pos:])
    return "".join(out)


def number_sections(s):
    """Number \\section and \\subsection as article does, letters after \\appendix."""
    sec, sub, appendix = 0, 0, False
    out, pos = [], 0
    pat = re.compile(r"\\(section|subsection|appendix)(?![A-Za-z])(\*?)")
    while True:
        m = pat.search(s, pos)
        if not m:
            break
        out.append(s[pos:m.start()])
        kind, starred = m.group(1), m.group(2)
        if kind == "appendix":
            appendix, sec, sub = True, 0, 0
            pos = m.end()
            continue
        title, j = group(s, m.end())
        if starred:
            out.append(s[m.start():j])
        else:
            if kind == "section":
                sec, sub = sec + 1, 0
                num = chr(ord("A") + sec - 1) if appendix else str(sec)
            else:
                sub += 1
                num = (chr(ord("A") + sec - 1) if appendix else str(sec)) + f".{sub}"
            out.append(f"\\{kind}*{{{num} {title}}}")
        pos = j
    out.append(s[pos:])
    return "".join(out)


def split_top(s, sep):
    """s split at sep where it is not inside braces."""
    parts, depth, cur, i = [], 0, [], 0
    while i < len(s):
        c = s[i]
        if depth == 0 and s.startswith(sep, i):  # before escapes: sep may be \\
            parts.append("".join(cur))
            cur, i = [], i + len(sep)
            continue
        if c == "\\":
            cur.append(s[i:i + 2])
            i += 2
            continue
        if c == "{":
            depth += 1
        elif c == "}":
            depth -= 1
        cur.append(c)
        i += 1
    parts.append("".join(cur))
    return parts


def flatten_header(tabular):
    """A tabular whose header is two rows joined by \\multicolumn, as one row.

    Markdown tables have one header row and no spans: pandoc writes such a
    tabular as HTML. "TPC-H" spanning "as written" and "gzip" becomes the two
    headers "TPC-H as written" and "TPC-H gzip". A table with spans in its body
    is left as it is.
    """
    tabular = re.sub(r"\\cmidrule(\([^)]*\))?\{[^}]*\}", "", tabular)
    head_at = tabular.find("\\midrule")
    if head_at < 0:
        return tabular
    top = tabular.find("\\toprule")
    if 0 <= top < head_at:
        start = top + len("\\toprule")
    else:
        start = group(tabular, len("\\begin{tabular}"))[1]  # past the column spec
    head, rest = tabular[start:head_at], tabular[head_at:]
    if "\\multicolumn" not in head or "\\multicolumn" in rest:
        return tabular
    rows = [r for r in split_top(head, "\\\\") if r.strip()]
    grid = []
    for r in rows:
        cells = []
        for c in split_top(r, "&"):
            c = c.strip()
            if c.startswith("\\multicolumn"):
                n, j = group(c, len("\\multicolumn"))
                _, j = group(c, j)
                text, _ = group(c, j)
                cells += [text.strip()] * int(n)
            else:
                cells.append(c)
        grid.append(cells)
    width = max(len(r) for r in grid)
    merged = []
    for col in range(width):
        parts = []
        for r in grid:
            p = r[col] if col < len(r) else ""
            if p and (not parts or parts[-1] != p):
                parts.append(p)
        merged.append(" ".join(parts))
    return tabular[:start] + "\n" + " & ".join(merged) + " \\\\\n" + rest


def flatten_tables(s):
    out, pos = [], 0
    for i, j, _ in environments(s, "tabular"):
        out.append(s[pos:i])
        out.append(flatten_header(s[i:j]))
        pos = j
    out.append(s[pos:])
    return "".join(out)


def floats(s, labels):
    """Replace figures and tables with their content and a numbered caption.

    Returns the text and the bodies of the figures, in order, for the images.
    Figure N becomes a paragraph FIGUREPLACEHOLDERN, made an image after pandoc.
    """
    figures, counts = [], {"figure": 0, "table": 0}
    pat = re.compile(r"\\begin\{(figure|table)\}(\[[^\]]*\])?")
    out, pos = [], 0
    while True:
        m = pat.search(s, pos)
        if not m:
            break
        env = m.group(1)
        end = s.index("\\end{" + env + "}", m.end())
        body = s[m.end():end]
        counts[env] += 1
        n = counts[env]

        caption, label = "", None
        ci = find_cmd(body, "caption")
        if ci >= 0:
            caption, cj = group(body, ci + len("\\caption"))
            body = body[:ci] + body[cj:]
        li = find_cmd(body, "label")
        if li >= 0:
            label, lj = group(body, li + len("\\label"))
            body = body[:li] + body[lj:]
            if labels.get(label) not in (None, str(n)):
                warn(f"{label} is {env} {n} here and {labels[label]} in main.aux: "
                     "is main.aux from this source?")

        name = "Figure" if env == "figure" else "Table"
        cap = f"\n\n\\textbf{{{name} {n}:}} {caption}\n\n" if caption else "\n\n"
        if env == "figure":
            figures.append(body)
            content = f"\n\nFIGUREPLACEHOLDER{n}\n\n"
        else:
            content = "\n\n" + body + "\n\n"
        out.append(s[pos:m.start()])
        out.append(content + cap)
        pos = end + len("\\end{" + env + "}")
    out.append(s[pos:])
    return "".join(out), figures


# ---- figures ----------------------------------------------------------------


def tectonic():
    for t in (os.path.join(HERE, "..", "tools", "bin", "tectonic"), shutil.which("tectonic")):
        if t and os.path.exists(t):
            return t
    return None


def render_figures(preamble, defs, figures, outdir, dpi):
    """Each figure body on a page of its own, cropped, then one PNG per page."""
    os.makedirs(outdir, exist_ok=True)
    pages = "\n".join(
        "\\begin{preview}\\begin{minipage}{\\linewidth}" + body + "\\end{minipage}\\end{preview}"
        for body in figures)
    doc = (preamble + "\n\\usepackage[active,tightpage]{preview}\n"
           "\\setlength\\PreviewBorder{6pt}\n" + defs + "\n\\begin{document}\n" + pages
           + "\n\\end{document}\n")
    with tempfile.TemporaryDirectory() as tmp:
        tex = os.path.join(tmp, "figures.tex")
        with open(tex, "w", encoding="utf-8") as f:
            f.write(doc)
        # The preamble and the plots read tables/ and tables-laptop/ relative to
        # paper/: pdflatex finds them from the working directory, tectonic only
        # on its search path (without it \IfFileExists is false and an axis is
        # left with no plot, which does not compile).
        if shutil.which("pdflatex"):
            cmd = ["pdflatex", "-interaction=nonstopmode", "-halt-on-error",
                   "-output-directory", tmp, tex]
        else:
            t = tectonic()
            if not t:
                die("no pdflatex and no tectonic: run ../tools/fetch-tectonic.sh")
            cmd = [t, "-Z", f"search-path={HERE}", "--outdir", tmp, tex]
        r = subprocess.run(cmd, cwd=HERE, capture_output=True, text=True)
        pdf = os.path.join(tmp, "figures.pdf")
        if r.returncode != 0 or not os.path.exists(pdf):
            sys.stderr.write(r.stdout[-3000:] + r.stderr[-3000:])
            shutil.copy(tex, os.path.join(outdir, "figures.tex"))
            die(f"the figures did not compile (above); the source is {outdir}/figures.tex")
        for old in os.listdir(outdir):
            if old.startswith("figure-") and old.endswith(".png"):
                os.remove(os.path.join(outdir, old))
        subprocess.run(["pdftocairo", "-png", "-r", str(dpi), pdf,
                        os.path.join(tmp, "page")], check=True)
        pngs = sorted(p for p in os.listdir(tmp) if p.startswith("page") and p.endswith(".png"))
        if len(pngs) != len(figures):
            die(f"{len(figures)} figures but {len(pngs)} pages rendered")
        magick = shutil.which("magick")
        for n, p in enumerate(pngs, 1):
            dst = os.path.join(outdir, f"figure-{n}.png")
            if magick:  # the preview is as wide as the text: trim it to the figure
                subprocess.run([magick, os.path.join(tmp, p), "-trim", "+repage",
                                "-bordercolor", "white", "-border", "20", dst], check=True)
            else:
                shutil.move(os.path.join(tmp, p), dst)


# ---- main -------------------------------------------------------------------


def main():
    ap = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    ap.add_argument("--out", default=os.path.join(HERE, "build", "md"),
                    help="output directory (default: build/md)")
    ap.add_argument("--dpi", type=int, default=200, help="resolution of the figures")
    ap.add_argument("--no-figures", action="store_true",
                    help="keep the figures already rendered in OUT/figures")
    args = ap.parse_args()

    for f in ("main.aux", "main.bbl"):
        if not os.path.exists(os.path.join(HERE, f)):
            die(f"no {f}: build the PDF first (make)")
    if not shutil.which("pandoc"):
        die("pandoc is not installed")

    main_tex = read(os.path.join(HERE, "main.tex"))
    preamble, rest = main_tex.split("\\begin{document}", 1)
    body = rest.split("\\end{document}", 1)[0]

    newest = max(os.path.getmtime(os.path.join(HERE, p))
                 for p in ["main.tex", "related-work.bib", "tables/numbers.tex",
                           "tables-laptop/numbers.tex"]
                 + ["sections/" + f for f in os.listdir(os.path.join(HERE, "sections"))]
                 if os.path.exists(os.path.join(HERE, p)))
    if os.path.getmtime(os.path.join(HERE, "main.aux")) < newest:
        warn("main.aux is older than the sources: references may be off (make first)")

    results = resultdefs(os.path.join(HERE, "tables", "numbers.tex"))
    results.update(resultdefs(os.path.join(HERE, "tables-laptop", "numbers.tex"), "laptop."))
    sweep_numbers = os.path.join(HERE, "tables-sweep", "numbers.tex")
    if os.path.exists(sweep_numbers):
        results.update(resultdefs(sweep_numbers, "sweep-run."))
    labels, cites = aux_refs(os.path.join(HERE, "main.aux"))
    final = re.findall(r"^\\final(true|false)", strip_comments(preamble), re.M)
    final = bool(final) and final[-1] == "true"

    title, _ = group(preamble, find_cmd(preamble, "title") + len("\\title"))
    author, _ = group(preamble, find_cmd(preamble, "author") + len("\\author"))

    s = strip_comments(body)
    s = inline_inputs(s)
    s = if_file_exists(s)
    s, macros, macro_source = collect_newcommands(s)

    # The figures, compiled with the paper's own preamble and the sections' macros.
    labels_seen = dict(labels)
    s, figures = floats(s, labels_seen)
    figdir = os.path.join(args.out, "figures")
    if figures and not args.no_figures:
        render_figures(preamble, macro_source, figures, figdir, args.dpi)

    # The rest is text: resolve what pandoc cannot.
    def table(dirname):
        def one(a):
            path = os.path.join(HERE, dirname, a[0] + ".tex")
            if not os.path.exists(path):
                warn(f"no table {dirname}/{a[0]}.tex")
                return f"\\textbf{{[{dirname}/{a[0]}.tex is not in this run]}}"
            return strip_comments(read(path))
        return one
    s = replace_cmd(s, "resulttable", 1, table("tables"))
    s = replace_cmd(s, "laptoptable", 1, table("tables-laptop"))
    s = expand_macros(s, macros)

    missing = set()
    def result(a):
        key = a[0].strip()
        if key not in results:
            missing.add(key)
            return f"\\textbf{{[{key}?]}}"
        return results[key]
    s = replace_cmd(s, "result", 1, result)
    if missing:
        warn("no result for: " + ", ".join(sorted(missing)))

    # {\let\textbf\relax \result{key}} prints a bold result plain (the
    # abstract); pandoc drops the whole group, so unwrap it here.
    s = re.sub(r"\{\\let\\textbf\\relax\s*\\textbf\{((?:[^{}]|\{[^{}]*\})*)\}\s*\}", r"\1", s)

    s = flatten_tables(s)
    # Markdown has no math: pandoc writes it as Unicode text, which bolds
    # \mathbf{1.25\times} digit by digit. Bold text with a × in it instead.
    # The math that follows stays math; a $ that closed it at once is dropped.
    s = re.sub(r"\$\\mathbf\{([^{}$]*?)\\times\}(?:\\!)?(\$)?",
               lambda m: f"\\textbf{{{m.group(1)}$\\times$}}" + ("" if m.group(2) else "$"), s)

    s = replace_cmd(s, "recheck", 1,
                    lambda a: "" if final else f" \\emph{{[check: {a[0]}]}}")

    banner = ""
    for key, who in (("provisional", ""), ("laptop.provisional", ", second machine")):
        if results.get(key):
            banner += f"\\textbf{{Provisional numbers{who}:}} {results[key]}.\n\n"
    s = s.replace("\\provisionalbanner", banner)
    s = s.replace("\\maketitle", "")

    unknown = set()
    def ref(a):
        if a[0] not in labels:
            unknown.add(a[0])
            return "??"
        return labels[a[0]]
    s = replace_cmd(s, "ref", 1, ref)
    def cite(a):
        keys = [k.strip() for k in a[0].split(",")]
        unknown.update(k for k in keys if k not in cites)
        return "[" + ", ".join(cites.get(k, "?") for k in keys) + "]"
    s = replace_cmd(s, "cite", 1, cite)
    if unknown:
        warn("not in main.aux: " + ", ".join(sorted(unknown)))

    s = re.sub(r"\\bibliographystyle\{[^}]*\}", "", s)
    s = replace_cmd(s, "bibliography", 1,
                    lambda a: bibliography(os.path.join(HERE, "main.bbl"), cites))
    s = s.replace("\\begin{abstract}", "\\section*{Abstract}").replace("\\end{abstract}", "")
    s = number_sections(s)
    # Run-in paragraph headings, as in the PDF, rather than a heading of their own.
    s = replace_cmd(s, "paragraph", 1, lambda a: f"\\textbf{{{a[0]}}} ")
    s = re.sub(r"\\label\{[^}]*\}", "", s)

    author = author.replace("\\small", "")
    doc = ("\\documentclass{article}\n"
           "\\newcommand{\\colbin}{colbin}\n"
           "\\newcommand{\\fmt}[1]{#1}\n"
           "\\newcommand{\\code}[1]{\\texttt{#1}}\n"
           "\\newcommand{\\tables}{tables}\n"
           "\\newcommand{\\FloatBarrier}{}\n"
           "\\newcommand{\\sloppy}{}\n"
           "\\begin{document}\n"
           "TITLEPLACEHOLDER\n\n" + author + "\n\n" + s + "\n\\end{document}\n")

    os.makedirs(args.out, exist_ok=True)
    # Math as Unicode text (2.3×, 2¹⁴ as 2<sup>14</sup>), not $...$, which only
    # some viewers render.
    r = subprocess.run(["pandoc", "-f", "latex", "-t", "gfm-tex_math_dollars-tex_math_gfm",
                        "--wrap=none", "--shift-heading-level-by=1"],
                       input=doc, capture_output=True, text=True)
    if r.returncode != 0:
        dbg = os.path.join(args.out, "paper.pandoc.tex")
        with open(dbg, "w", encoding="utf-8") as f:
            f.write(doc)
        sys.stderr.write(r.stderr)
        die(f"pandoc failed; its input is {dbg}")
    if r.stderr.strip():
        sys.stderr.write(r.stderr)
    md = r.stdout

    t = subprocess.run(["pandoc", "-f", "latex", "-t", "gfm", "--wrap=none"],
                       input=title.replace("\\\\", " "), capture_output=True, text=True,
                       check=True).stdout.strip()
    md = md.replace("TITLEPLACEHOLDER", "# " + t, 1)
    for n in range(1, len(figures) + 1):
        md = md.replace(f"FIGUREPLACEHOLDER{n}", f"![Figure {n}](figures/figure-{n}.png)", 1)
    left = re.findall(r"(TITLE|FIGURE)PLACEHOLDER\d*", md)
    if left:
        die(f"placeholders left in the Markdown: {left}")

    out = os.path.join(args.out, "paper.md")
    with open(out, "w", encoding="utf-8") as f:
        f.write(md)
    print(f"{os.path.relpath(out)}: {len(figures)} figures in "
          f"{os.path.relpath(figdir)}/")


if __name__ == "__main__":
    main()
