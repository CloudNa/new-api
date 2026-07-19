import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch

import server


class CleanupPolicyTests(unittest.TestCase):
    def test_defaults_are_conservative(self) -> None:
        policy = server.cleanup_policy()

        self.assertEqual(policy["keep_rollback_images"], 5)
        self.assertEqual(policy["keep_backups"], 5)
        self.assertEqual(policy["build_cache_max_age_hours"], 168)
        self.assertTrue(policy["prune_dangling_images"])
        self.assertTrue(policy["prune_build_cache"])

    def test_rejects_unsafe_retention_values(self) -> None:
        with self.assertRaises(ValueError):
            server.cleanup_policy({"keep_rollback_images": 0})
        with self.assertRaises(ValueError):
            server.cleanup_policy({"keep_backups": 21})
        with self.assertRaises(ValueError):
            server.cleanup_policy({"build_cache_max_age_hours": 1})
        with self.assertRaises(ValueError):
            server.cleanup_policy({"prune_build_cache": "yes"})


class CleanupPreviewTests(unittest.TestCase):
    def test_preview_keeps_latest_images_and_backups_per_component(self) -> None:
        original_backup_dir = server.BACKUP_DIR
        original_root = server.ROOT
        try:
            with tempfile.TemporaryDirectory() as temp_dir:
                root = Path(temp_dir)
                server.ROOT = root
                server.BACKUP_DIR = root / "backups"
                component_dir = server.BACKUP_DIR / "new-api"
                for backup_id in (
                    "20260705-000000",
                    "20260704-000000",
                    "20260703-000000",
                    "20260702-000000",
                ):
                    target = component_dir / backup_id
                    target.mkdir(parents=True)
                    (target / "manifest.env").write_text("COMPONENT=new-api\n", encoding="utf-8")
                (component_dir / "manual-notes").mkdir()

                image_rows = [
                    {
                        "Repository": "glart/rollback-new-api",
                        "Tag": tag,
                        "CreatedAt": f"2026-07-{day} 00:00:00 +0000 UTC",
                    }
                    for day, tag in (("05", "20260705-000000"), ("04", "20260704-000000"), ("03", "20260703-000000"))
                ]
                image_rows.append(
                    {
                        "Repository": "glart/rollback-new-api",
                        "Tag": "latest",
                        "CreatedAt": "2026-07-06 00:00:00 +0000 UTC",
                    }
                )
                docker_rows = [
                    {
                        "Type": "Images",
                        "TotalCount": "10",
                        "Active": "2",
                        "Size": "10GB",
                        "Reclaimable": "5GB (50%)",
                    },
                    {
                        "Type": "Build Cache",
                        "TotalCount": "20",
                        "Active": "0",
                        "Size": "20GB",
                        "Reclaimable": "15GB",
                    },
                ]

                def fake_docker_json_lines(args: list[str]) -> list[dict]:
                    if args[:2] == ["image", "ls"]:
                        return image_rows
                    if args[:2] == ["system", "df"]:
                        return docker_rows
                    return []

                with patch.object(server, "docker_json_lines", side_effect=fake_docker_json_lines), patch.object(
                    server.shutil,
                    "disk_usage",
                    return_value=SimpleNamespace(total=1000, used=800, free=200),
                ):
                    preview = server.cleanup_preview(
                        server.cleanup_policy(
                            {
                                "keep_rollback_images": 1,
                                "keep_backups": 2,
                                "build_cache_max_age_hours": 168,
                            }
                        )
                    )

                self.assertEqual(preview["rollback_images"]["candidate_count"], 2)
                self.assertEqual(preview["backups"]["candidate_count"], 2)
                self.assertGreater(preview["backups"]["estimated_bytes"], 0)
                self.assertEqual(preview["docker"]["build_cache"]["reclaimable"], "15GB")
                self.assertEqual(preview["disk"]["free_bytes"], 200)
        finally:
            server.BACKUP_DIR = original_backup_dir
            server.ROOT = original_root

    def test_preview_ignores_non_rollback_repositories_and_tags(self) -> None:
        with patch.object(
            server,
            "docker_json_lines",
            return_value=[
                {"Repository": "glart/new-api", "Tag": "latest"},
                {"Repository": "glart/rollback-new-api", "Tag": "latest"},
                {"Repository": "glart/rollback-new-api", "Tag": "20260701-000000"},
            ],
        ):
            self.assertEqual(len(server.rollback_image_candidates(1)), 0)

    def test_cleanup_failure_releases_running_state(self) -> None:
        policy = server.cleanup_policy()
        with patch.object(server, "cleanup_preview", side_effect=RuntimeError("disk unavailable")):
            server.run_cleanup_operation(policy)

        self.assertFalse(server.state["running"])
        self.assertEqual(server.state["last_exit"], 1)
        self.assertIn("disk unavailable", server.state["message"])


if __name__ == "__main__":
    unittest.main()
