"""Estimate expected score improvement from previous comparable attempts."""
from __future__ import annotations

from dataclasses import dataclass

import numpy as np
import pandas as pd


def history(frame: pd.DataFrame) -> pd.DataFrame:
    """Each attempt may see only strictly earlier scores in its user/quiz/variant."""
    result = pd.DataFrame(index=frame.index, columns=["history_n", "prior", "gain"], dtype=float)
    for _, group in frame.groupby(["user_id", "quiz_id", "variant_id"], sort=False):
        past: list[float] = []
        for _, block in group.groupby("submitted_at", sort=True):
            if past:
                prior = float(np.mean(past))
                result.loc[block.index, "history_n"] = len(past)
                result.loc[block.index, "prior"] = prior
                result.loc[block.index, "gain"] = block.score_percent - prior
            past.extend(block.score_percent)
    return result


@dataclass
class GainRule:
    """Robust normal gain and noise by #previous sittings and prior-score band."""
    global_stats: tuple[float, float]
    by_sitting: dict[int, tuple[float, float]]
    by_band: dict[tuple[int, int], tuple[float, float]]

    @classmethod
    def fit(cls, rows: pd.DataFrame, minimum: int = 30) -> GainRule:
        source = rows.dropna(subset=["history_n", "prior", "gain"]).copy()
        if source.empty:
            raise ValueError("Need normal training attempts with history")
        source["band"] = np.minimum(source.prior // 20, 4).astype(int)

        def stats(group: pd.DataFrame) -> tuple[float, float]:
            location = float(group.gain.median())
            return location, max(5.0, 1.4826 * float((group.gain - location).abs().median()))

        return cls(stats(source),
                   {int(k): stats(g) for k, g in source.groupby("history_n") if len(g) >= minimum},
                   {(int(n), int(b)): stats(g) for (n, b), g in source.groupby(["history_n", "band"])
                    if len(g) >= minimum})

    def score(self, rows: pd.DataFrame) -> np.ndarray:
        result = np.full(len(rows), np.nan)
        for i, r in enumerate(rows.itertuples()):
            if np.isfinite(r.history_n):
                n, b = int(r.history_n), min(4, int(r.prior // 20))
                gain, scale = self.by_band.get((n, b), self.by_sitting.get(n, self.global_stats))
                result[i] = (r.gain - gain) / scale
        return result
