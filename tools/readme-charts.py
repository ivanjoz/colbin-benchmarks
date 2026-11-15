#!/usr/bin/env python3
"""The two figures of colbin's README, drawn from the row-count sweep.

    tools/readme-charts.py [results/paper-v1-sweep] [../colbin/docs]

Reads the sweep's benchmark output (bench-web.txt, bench-bulk.txt: one table
cut to its first 1, 2, 4, ... rows) and writes, for a light and a dark page:

    rows-time-<mode>.svg     decode-fresh, ns per row, by rows (lineitem, Bimbo)
    rows-memory-<mode>.svg   allocations per decode by rows, and bytes allocated
                             at the largest cut (lineitem)

Every number is the median of the case's samples. Standard library only.
"""
import collections
import math
import re
import statistics
import sys

RUN = sys.argv[1] if len(sys.argv) > 1 else "results/paper-v1-sweep"
OUT = sys.argv[2] if len(sys.argv) > 2 else "../colbin/docs"

# Format, legend name, and its colour on a light and on a dark page. The order
# is the palette's: a format keeps its colour in every figure.
SERIES = [
    ("colbin", "colbin", "#2a78d6", "#3987e5"),
    ("vtprotobuf", "vtprotobuf", "#eb6834", "#d95926"),
    ("msgpack", "msgpack", "#1baf7a", "#199e70"),
    ("arrow", "Arrow", "#eda100", "#c98500"),
    ("json-v2", "JSON", "#e87ba4", "#d55181"),
]
INK = {
    "light": dict(primary="#0b0b0b", secondary="#52514e", muted="#898781", grid="#e1e0d9", axis="#c3c2b7"),
    "dark": dict(primary="#f0efec", secondary="#c3c2b7", muted="#898781", grid="#2c2c2a", axis="#383835"),
}
FONT = 'system-ui, -apple-system, "Segoe UI", sans-serif'

LINE = re.compile(
    r"BenchmarkColbin/sweep/(\w+)/(\d+)/([\w+-]+)/decode-fresh\s+\d+\s+[\d.]+ ns/op\s+"
    r"([\d.]+) ns/row\s+(\d+) B/op\s+(\d+) allocs/op")


def load():
    """(table, format) -> {rows: (ns per row, bytes, allocations)}, medians."""
    samples = collections.defaultdict(list)
    for name in ("bench-web.txt", "bench-bulk.txt"):
        try:
            lines = open(f"{RUN}/{name}")
        except FileNotFoundError:
            continue
        for line in lines:
            m = LINE.match(line)
            if m:
                samples[(m[1], m[3], int(m[2]))].append((float(m[4]), int(m[5]), int(m[6])))
    out = collections.defaultdict(dict)
    for (table, fmt, rows), values in samples.items():
        out[(table, fmt)][rows] = tuple(statistics.median(v[i] for v in values) for i in range(3))
    return out


def thousands(n):
    return f"{int(round(n)):,}".replace(",", " ")


def short(n):
    if n >= 1e6:
        return f"{n / 1e6:.1f}M".replace(".0M", "M")
    if n >= 1e4:
        return f"{n / 1e3:.0f}k"
    return thousands(n)


class Svg:
    def __init__(self, width, height, mode):
        self.w, self.h, self.mode, self.ink = width, height, mode, INK[mode]
        self.parts, self.clips = [], 0

    def add(self, text):
        self.parts.append(text)

    def text(self, x, y, s, ink="secondary", size=13, anchor="start", weight=400):
        self.add(f'<text x="{x:.1f}" y="{y:.1f}" fill="{self.ink[ink]}" font-size="{size}" '
                 f'text-anchor="{anchor}" font-weight="{weight}">{s}</text>')

    def colour(self, index):
        return SERIES[index][2 if self.mode == "light" else 3]

    def legend(self, x, y):
        for i, (_, name, _, _) in enumerate(SERIES):
            self.add(f'<line x1="{x}" y1="{y - 4}" x2="{x + 18}" y2="{y - 4}" stroke="{self.colour(i)}" '
                     f'stroke-width="{3.5 if i == 0 else 2.5}" stroke-linecap="round"/>')
            self.text(x + 25, y, name, "primary" if i == 0 else "secondary", weight=600 if i == 0 else 400)
            x += 43 + 7.4 * len(name)

    def render(self, label):
        return (f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {self.w} {self.h}" width="{self.w}" '
                f'height="{self.h}" role="img" aria-label="{label}" font-family=\'{FONT}\'>\n'
                + "\n".join(self.parts) + "\n</svg>\n")


def line_panel(svg, data, table, column, x0, title, unit, yrange, yticks, value_label, plot_w=300):
    """One log-log panel: a line per format against the rows of the message."""
    top, plot_h = 78, 300
    left = x0 + 54
    rows = sorted(data[(table, "colbin")])
    xlo, xhi = math.log2(rows[0]), math.log2(rows[-1])
    ylo, yhi = math.log10(yrange[0]), math.log10(yrange[1])
    px = lambda n: left + (math.log2(n) - xlo) / (xhi - xlo) * plot_w
    py = lambda v: top + plot_h - (math.log10(v) - ylo) / (yhi - ylo) * plot_h

    svg.text(x0 + 8, 46, title, "primary", 14, weight=600)
    svg.text(x0 + 8, 64, unit, "muted", 12)
    for tick in yticks:
        y = py(tick)
        svg.add(f'<line x1="{left}" y1="{y:.1f}" x2="{left + plot_w}" y2="{y:.1f}" stroke="{svg.ink["grid"]}"/>')
        svg.text(left - 8, y + 4, short(tick), "muted", 12, "end")
    svg.add(f'<line x1="{left}" y1="{top + plot_h}" x2="{left + plot_w}" y2="{top + plot_h}" stroke="{svg.ink["axis"]}"/>')
    for n in (1, 8, 64, 512, 4096, 32768, 262144):
        if n <= rows[-1]:
            svg.text(px(n), top + plot_h + 18, short(n) if n < 4096 else f"{n // 1024}k", "muted", 12, "middle")
    svg.text(left + plot_w / 2, top + plot_h + 40, "rows in the message", "secondary", 12, "middle")

    # The threshold from which a slice is written as columns.
    x8 = px(8)
    svg.add(f'<line x1="{x8:.1f}" y1="{top}" x2="{x8:.1f}" y2="{top + plot_h}" stroke="{svg.ink["muted"]}" '
            f'stroke-dasharray="4 4"/>')
    svg.text(x8 + 6, top + 12, "8 rows: a table from here", "muted", 11)

    svg.clips += 1
    clip = f"clip{svg.clips}"
    svg.add(f'<clipPath id="{clip}"><rect x="{left}" y="{top}" width="{plot_w + 6}" height="{plot_h}"/></clipPath>')
    ends = []
    for i in reversed(range(len(SERIES))):  # colbin drawn last, on top
        fmt = SERIES[i][0]
        series = data[(table, fmt)]
        points = " ".join(f"{px(n):.1f},{py(series[n][column]):.1f}" for n in rows if n in series)
        svg.add(f'<polyline points="{points}" fill="none" stroke="{svg.colour(i)}" clip-path="url(#{clip})" '
                f'stroke-width="{3.5 if i == 0 else 2}" stroke-linejoin="round" stroke-linecap="round"/>')
        last = series[rows[-1]][column]
        ends.append([py(last), i, last])

    # Direct labels at the line ends, pushed apart where they would overlap.
    ends.sort()
    for k in range(1, len(ends)):
        ends[k].append(max(ends[k][0], ends[k - 1][-1] + 16) if k > 1 else max(ends[k][0], ends[0][0] + 16))
        if k == 1:
            ends[0].append(ends[0][0])
    xe = left + plot_w
    for y, i, last, ly in ends:
        svg.add(f'<circle cx="{xe:.1f}" cy="{y:.1f}" r="4.5" fill="{svg.colour(i)}"/>')
        if abs(ly - y) > 1:
            svg.add(f'<line x1="{xe + 6}" y1="{y:.1f}" x2="{xe + 12}" y2="{ly:.1f}" stroke="{svg.ink["axis"]}"/>')
        name = SERIES[i][1]
        svg.add(f'<text x="{xe + 15}" y="{ly + 4:.1f}" font-size="12" fill="{svg.ink["secondary"]}">'
                f'<tspan font-weight="{700 if i == 0 else 400}" fill="{svg.ink["primary"] if i == 0 else svg.ink["secondary"]}">{name}</tspan>'
                f'<tspan dx="5" font-weight="600" fill="{svg.ink["primary"]}">{value_label(last)}</tspan></text>')


def bar_panel(svg, data, table, rows, x0, title, unit):
    """Bytes allocated to decode the largest cut, one bar per format."""
    top, bar_h, gap, label_w, plot_w = 92, 22, 22, 92, 250
    left = x0 + label_w
    values = sorted(((data[(table, fmt)][rows][1] / 1e6, i) for i, (fmt, *_) in enumerate(SERIES)))
    scale = plot_w / (math.ceil(values[-1][0] / 100) * 100)
    svg.text(x0 + 8, 46, title, "primary", 14, weight=600)
    svg.text(x0 + 8, 64, unit, "muted", 12)
    bottom = top + len(values) * (bar_h + gap) - gap + 12
    for tick in range(0, int(values[-1][0] / 100) + 2):
        x = left + tick * 100 * scale
        svg.add(f'<line x1="{x:.1f}" y1="{top - 10}" x2="{x:.1f}" y2="{bottom}" stroke="{svg.ink["grid"]}"/>')
        svg.text(x, bottom + 18, f"{tick * 100}", "muted", 12, "middle")
    svg.text(left + plot_w / 2, bottom + 40, "MB allocated per decode", "secondary", 12, "middle")
    svg.add(f'<line x1="{left}" y1="{top - 10}" x2="{left}" y2="{bottom}" stroke="{svg.ink["axis"]}"/>')
    for k, (mb, i) in enumerate(values):
        y = top + k * (bar_h + gap)
        w = mb * scale
        # Square at the baseline, rounded at the data end.
        svg.add(f'<path d="M{left},{y} h{w - 4:.1f} a4,4 0 0 1 4,4 v{bar_h - 8} a4,4 0 0 1 -4,4 h-{w - 4:.1f} z" '
                f'fill="{svg.colour(i)}"/>')
        svg.text(left - 10, y + bar_h / 2 + 4, SERIES[i][1], "primary" if i == 0 else "secondary", 13, "end",
                 700 if i == 0 else 400)
        svg.text(left + w + 8, y + bar_h / 2 + 4, f"{mb:.0f} MB", "primary", 12, weight=600)


def main():
    data = load()
    if ("lineitem", "colbin") not in data:
        sys.exit(f"{RUN}: no sweep/lineitem cases")
    top_rows = max(data[("lineitem", "colbin")])
    for mode in ("light", "dark"):
        svg = Svg(960, 430, mode)
        svg.legend(8, 18)
        ticks = (20, 50, 100, 200, 500, 1000, 2000)
        ns = lambda v: f"{v:.0f} ns"
        line_panel(svg, data, "lineitem", 0, 0, "TPC-H lineitem, 16 columns", "decode, nanoseconds per row",
                   (14, 2600), ticks, ns)
        line_panel(svg, data, "bimbo", 0, 490, "Public BI Bimbo, 12 columns", "decode, nanoseconds per row",
                   (14, 2600), ticks, ns)
        open(f"{OUT}/rows-time-{mode}.svg", "w").write(svg.render(
            "Nanoseconds per row to decode one table cut to 1 up to 524 288 rows, by format"))

        svg = Svg(960, 430, mode)
        svg.legend(8, 18)
        line_panel(svg, data, "lineitem", 2, 0, "Allocations to decode TPC-H lineitem", "allocations per decode",
                   (1, 6e6), (1, 10, 100, 1000, 10000, 100000, 1000000), short, plot_w=330)
        bar_panel(svg, data, "lineitem", top_rows, 540, f"Memory to decode {thousands(top_rows)} rows",
                  "bytes allocated, decoding into a fresh value")
        open(f"{OUT}/rows-memory-{mode}.svg", "w").write(svg.render(
            "Allocations per decode by rows, and bytes allocated to decode the largest table, by format"))

    # The numbers the README quotes beside the figures.
    for table in ("lineitem", "bimbo"):
        for rows in sorted(data[(table, "colbin")]):
            print(table, rows, *(f"{fmt}={data[(table, fmt)][rows][0]:.0f}ns/{data[(table, fmt)][rows][2]:.0f}a"
                                 for fmt, *_ in SERIES))


if __name__ == "__main__":
    main()
