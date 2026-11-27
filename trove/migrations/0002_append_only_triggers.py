from django.db import migrations

TABLES = (
    "trove_item",
    "trove_blob",
    "trove_claim",
    "trove_link",
    "trove_activity",
)


def _triggers():
    operations = []
    for table in TABLES:
        operations.append(
            migrations.RunSQL(
                sql=[
                    (
                        f"CREATE TRIGGER {table}_no_update BEFORE UPDATE ON {table} "
                        f"BEGIN SELECT RAISE(ABORT, 'append-only'); END;"
                    ),
                    (
                        f"CREATE TRIGGER {table}_no_delete BEFORE DELETE ON {table} "
                        f"BEGIN SELECT RAISE(ABORT, 'append-only'); END;"
                    ),
                ],
                reverse_sql=[
                    f"DROP TRIGGER IF EXISTS {table}_no_update;",
                    f"DROP TRIGGER IF EXISTS {table}_no_delete;",
                ],
                hints={"target_db": "data"},
            )
        )
    return operations


class Migration(migrations.Migration):
    dependencies = [
        ("trove", "0001_initial"),
    ]

    operations = _triggers()
