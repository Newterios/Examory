import hashlib
import tempfile
import unittest
from pathlib import Path

import cv2
import numpy as np

from ml.proctoring.detect import analyze, segments


class PresenceTests(unittest.TestCase):
    def test_short_gap_and_different_states(self):
        self.assertEqual(
            segments(
                [(0, 0), (600, 1), (800, 0), (1400, 2), (2000, 1)], 2300, 500, 250
            ),
            [
                {"from_ms": 0, "to_ms": 1400, "kind": "absent"},
                {"from_ms": 1400, "to_ms": 2000, "kind": "multiple_faces"},
            ],
        )
        self.assertEqual(segments([(0, 1), (100, 0), (150, 1)], 200, 100, 0), [])

    def test_real_decoder_on_synthetic_blank_recording(self):
        with tempfile.TemporaryDirectory() as temp:
            video = Path(temp) / "blank.avi"
            writer = cv2.VideoWriter(
                str(video), cv2.VideoWriter_fourcc(*"MJPG"), 10, (160, 120)
            )
            self.assertTrue(writer.isOpened())
            for _ in range(20):
                writer.write(np.zeros((120, 160, 3), dtype=np.uint8))
            writer.release()
            r = {
                "exam_session_id": "session_fixture",
                "webcam_consent": False,
                "context": {
                    "course_id": None,
                    "group_id": None,
                    "session_id": "session_fixture",
                },
                "video_sha256": hashlib.sha256(video.read_bytes()).hexdigest(),
            }
            with self.assertRaises(ValueError):
                analyze(video, r)
            r["webcam_consent"] = True
            result = analyze(video, r)
            self.assertEqual(
                result["risk"]["payload"]["evidence"]["segments"],
                [{"kind": "absent", "from_ms": 0, "to_ms": 2000}],
            )
            self.assertEqual(result["risk"]["payload"]["score"], 1)
