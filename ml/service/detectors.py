"""Published model names, stored raw scores and quantile-rule explanations."""
from __future__ import annotations

from dataclasses import dataclass

import numpy as np
import pandas as pd

from ml.anomaly.features import LOG_FEATURES
from ml.anomaly.train import matrix

NAMES = {"iforest": "isolation-forest-v1", "quantile_rules": "quantile-rules-v1"}
DEFAULT_MODEL = "quantile_rules"


@dataclass(frozen=True)
class ModelScore:
    key: str
    version: str
    raw: float
    level: str
    shadow: bool


def explain_rules(frame: pd.DataFrame, model: dict, raw: float) -> str:
    """Describe the dimensions meeting the frozen two-sided midrank cutoff.

    Boundary inclusivity depends on repeated training values. Determine it
    from the rank at the boundary, using the same tie rule as quantile_scores.
    """
    columns = model["columns"]
    values, _ = matrix(frame, columns, model["medians"])
    refs = model["sorted_train"]
    threshold = model["medium_threshold"]
    n = len(refs)
    # Gap ranks g/n also describe the two outermost open intervals (g=0,n).
    gap_ranks = np.arange(n + 1) / n
    outside = 2 * np.abs(gap_ranks - .5) >= threshold
    low_index = np.flatnonzero(outside & (gap_ranks <= .5))[-1]
    high_index = np.flatnonzero(outside & (gap_ranks >= .5))[0] - 1
    observations = []
    for i, x in enumerate(values[0]):
        ref = refs[:, i]
        def rank(v: float, reference: np.ndarray = ref) -> float:
            return (reference.searchsorted(v, "left") + reference.searchsorted(v, "right")) / (2 * n)
        r = rank(x)
        if 2 * abs(r - .5) < threshold:
            continue
        below = r <= .5
        bound = ref[low_index if below else high_index]
        boundary_rank = rank(bound)
        inclusive = 2 * abs(boundary_rank - .5) >= threshold and ((boundary_rank <= .5) == below)
        op = ("<" if below else ">") + ("=" if inclusive else "")
        column = columns[i % len(columns)]
        name = column
        if i >= len(columns):
            name += ".missing"
        else:
            if column in LOG_FEATURES:
                x, bound = np.expm1(x), np.expm1(bound)
            if pd.isna(frame.iloc[0][column]):
                name += " (imputed)"
        observations.append(f"{name}={x:.8g} (rule {op} {bound:.8g}, rank {r:.6g})")
    return (f"Two-sided quantile rules: raw score {raw:.8g}, alert cutoff {threshold:.8g}. "
            + "; ".join(observations)
            + ". Relative anomaly ranking, not a probability or proof of misconduct.")
