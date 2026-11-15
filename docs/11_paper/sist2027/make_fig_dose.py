"""Publication version of F8 (fi-dose heat-maps) for the SIST 2027 paper.

Run from the repository root:
    .venv/bin/python docs/11_paper/sist2027/make_fig_dose.py

Cell means and "95 % interval includes zero" flags come from the same code path that builds
F8 and tables/fi_dose.md (analyze.load -> review_analysis.fi_dose_summary); nothing is
re-computed here. The script checks every cell against tables/fi_dose.md before plotting.
"""
from __future__ import annotations

import re
import sys
from pathlib import Path

import numpy as np

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT))

from ml.experiments.inflation import analyze, review_analysis  # noqa: E402
from ml.experiments.inflation.common import DETECTORS, SLOTS  # noqa: E402

RESULTS = ROOT / "ml/experiments/inflation/results"
OUT = Path(__file__).resolve().parent / "fig_dose.pdf"

ROW_ORDER = ("quantile_rules", "iforest_no_peers", "iforest")
ROW_LABEL = {"quantile_rules": "Rules", "iforest_no_peers": "IF", "iforest": "IF+peers"}
COL_TITLE = {"S1": "Pre-knowledge", "S2": "Look-up", "S3": "Peer copying", "S4": "Proxy"}
WIDTH, HEIGHT = 7.16, 3.85


def load_summary() -> dict:
    """Same loading as analyze.py's run (read_json + expand_stage), then fi_dose_summary."""
    def load(name: str) -> dict:
        return analyze.expand_stage(analyze.read_json(RESULTS / "stages" / f"{name}.json"))
    return review_analysis.fi_dose_summary(load("fi_dose"), load("main"), load("dose"))


def includes_zero(cell: dict) -> bool:
    """Exactly F8's convention: significant iff low > 0 or high < 0; otherwise marked with a circle."""
    return not (cell["low"] is not None and (cell["low"] > 0 or cell["high"] < 0))


def parse_table() -> dict:
    text = (RESULTS / "tables" / "fi_dose.md").read_text(encoding="utf-8")
    text = text.split("## Δ = recall_FI(λ) − recall_BL(x)", 1)[1].split("\n## ", 1)[0]
    inverse = {v: k for k, v in review_analysis.LABEL.items()}
    out, current = {}, None
    for line in text.splitlines():
        m = re.match(r"### (.+), (S\d) ", line)
        if m:
            current = (inverse[m.group(1)], m.group(2))
            continue
        m = re.match(r"\| λ = ([\d.]+) \|(.*)\|\s*$", line)
        if m and current:
            for k, cell in enumerate(c.strip() for c in m.group(2).split("|")):
                mean = float(cell.split(" [")[0])
                out[(*current, float(m.group(1)), k)] = (mean, not cell.endswith("*"))
    return out


def verify(summary: dict) -> None:
    table = parse_table()
    lambdas, xs = summary["lambdas"], summary["intensities"]
    print(f"{'detector':<17}{'type':<4}{'lambda':>7}" + "".join(f"{review_analysis._g(x):>10}" for x in xs))
    max_diff, mismatched, n = 0.0, 0, 0
    for d in ROW_ORDER:
        for s in SLOTS:
            for lam in lambdas:
                row = []
                for k, x in enumerate(xs):
                    c = summary["delta"][f"{d}|slot:{s}|{lam}|{x}"]
                    value, zero = 100 * c["mean"], includes_zero(c)
                    t_value, t_zero = table[(d, s, float(lam), k)]
                    max_diff = max(max_diff, abs(value - t_value))
                    mismatched += zero != t_zero
                    n += 1
                    row.append(f"{value:9.3f}{'o' if zero else ' '}")
                print(f"{ROW_LABEL[d]:<17}{s:<4}{lam:>7g}" + "".join(row))
    print(f"\ncells compared: {n} (expected {len(ROW_ORDER) * len(SLOTS) * len(lambdas) * len(xs)}); "
          f"max |figure - tables/fi_dose.md| = {max_diff:.6f} pp; zero-interval flag mismatches: {mismatched}")
    assert n == len(table) == 288 and max_diff < 0.05 and mismatched == 0


def pick_font() -> str:
    from matplotlib import font_manager
    for name in ("Times New Roman", "STIXGeneral", "Nimbus Roman", "DejaVu Serif"):
        try:
            font_manager.findfont(name, fallback_to_default=False)
            return name
        except ValueError:
            continue
    return "serif"


def plot(summary: dict, png: Path | None = None) -> None:
    plt = analyze._plt()
    from matplotlib import patheffects  # Agg backend + the project's rcParams (pdf.fonttype 42)
    font = pick_font()
    plt.rcParams.update({"font.family": "serif", "font.serif": [font], "mathtext.fontset": "stix",
                         "pdf.fonttype": 42, "ps.fonttype": 42, "savefig.bbox": "standard",
                         "axes.linewidth": 0.6, "xtick.major.width": 0.6, "ytick.major.width": 0.6,
                         "xtick.major.size": 2, "ytick.major.size": 2, "xtick.major.pad": 1.5,
                         "ytick.major.pad": 1.5})
    lambdas, xs = summary["lambdas"], summary["intensities"]
    # identical colour limit to F8: max |mean| over every delta cell of the summary
    limit = max(abs(c["mean"]) for c in summary["delta"].values() if c["mean"] is not None) * 100
    fig = plt.figure(figsize=(WIDTH, HEIGHT))
    left, right, bottom, top = 0.072, 0.905, 0.095, 0.94
    fig.subplots_adjust(left=left, right=right, bottom=bottom, top=top, wspace=0.07, hspace=0.12)
    axes = fig.subplots(len(ROW_ORDER), len(SLOTS), squeeze=False)
    numeric = [k for k, x in enumerate(xs) if x != "default"]
    contour_drawn = 0
    for i, d in enumerate(ROW_ORDER):
        for j, s in enumerate(SLOTS):
            cells = [[summary["delta"][f"{d}|slot:{s}|{lam}|{x}"] for x in xs] for lam in lambdas]
            grid = np.array([[100 * (c["mean"] or 0) for c in row] for row in cells])  # as in F8
            ax = axes[i][j]
            image = ax.imshow(grid, cmap="RdBu_r", vmin=-limit, vmax=limit, origin="lower", aspect="auto")
            part = grid[:, numeric]
            if part.min() < 0 < part.max():  # F8: zero contour over the ordered intensity grid only
                ax.contour(numeric, range(len(lambdas)), part, levels=[0], colors="black", linewidths=0.8)
                contour_drawn += 1
            if len(numeric) < len(xs):
                ax.axvline(len(numeric) - 0.5, color="black", lw=1.0)
            for p in range(len(lambdas)):
                for q in range(len(xs)):
                    v = grid[p, q]
                    label = f"{v:.0f}".replace("-", "−")
                    if label == "−0":
                        label = "0"
                    colour = "white" if abs(v) > 0.55 * limit else "black"
                    # halo in the cell's own colour hides the zero contour behind the digits
                    halo = [patheffects.withStroke(linewidth=2.0, foreground=image.cmap(image.norm(v)))]
                    ax.text(q, p, label + ("°" if includes_zero(cells[p][q]) else ""), ha="center",
                            va="center", fontsize=6.5, color=colour, zorder=5, path_effects=halo)
            ax.set_xticks(range(len(xs)))
            ax.set_yticks(range(len(lambdas)))
            ax.tick_params(labelsize=7)
            if i == len(ROW_ORDER) - 1:
                ax.set_xticklabels([review_analysis._g(x) for x in xs])
            else:
                ax.set_xticklabels([])
            if j == 0:
                ax.set_yticklabels([f"{v:g}" for v in lambdas])
                ax.set_ylabel("Injection dose λ", fontsize=7, labelpad=2)
                ax.annotate(ROW_LABEL[d], xy=(0, 0.5), xycoords="axes fraction", xytext=(-33, 0),
                            textcoords="offset points", rotation=90, ha="center", va="center",
                            fontsize=8, fontweight="bold")
            else:
                ax.set_yticklabels([])
            if i == 0:
                ax.set_title(COL_TITLE[s], fontsize=8, pad=3)
            for spine in ax.spines.values():
                spine.set_visible(True)
    # one x label centred under the bottom row
    fig.text((left + right) / 2, 0.018, "Behavior-level intensity x", ha="center", va="bottom", fontsize=7)
    cax = fig.add_axes([right + 0.012, bottom, 0.012, top - bottom])
    cbar = fig.colorbar(image, cax=cax)
    cbar.ax.tick_params(labelsize=7, width=0.6, length=2)
    cbar.outline.set_linewidth(0.6)
    cbar.set_label("Recall difference (percentage points)", fontsize=7)
    OUT.parent.mkdir(parents=True, exist_ok=True)
    fig.savefig(OUT, metadata={"CreationDate": None})
    if png:  # optional raster check copy, same figure rendered by Agg
        fig.savefig(png, dpi=200)
    w, h = fig.get_size_inches()
    print(f"wrote {OUT.relative_to(ROOT)}: {w:.2f} x {h:.2f} in; font: {font}; "
          f"colour limit ±{limit:.2f} pp; zero contour drawn in {contour_drawn}/12 panels")
    plt.close(fig)


def main() -> None:
    summary = load_summary()
    assert tuple(DETECTORS) and set(ROW_ORDER) == set(DETECTORS)
    verify(summary)
    png = Path(sys.argv[sys.argv.index("--png") + 1]) if "--png" in sys.argv else None
    plot(summary, png)


if __name__ == "__main__":
    main()
