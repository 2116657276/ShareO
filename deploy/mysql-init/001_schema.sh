#!/bin/bash
# RETIRED legacy MySQL bootstrap. Native runtime applies migrations/postgres/*.sql.
# Initialize the clean-slate lightweight schema and additive migrations.
# Demo data is loaded separately.
# The official image sources this file into its entrypoint. Do not enable
# nounset here: it would leak into the parent script and break optional
# MYSQL_* variables after this initializer returns.
set -eo pipefail

export MYSQL_PWD="${MYSQL_ROOT_PASSWORD}"

for migration in /migrations/*.sql; do
    mysql -uroot "${MYSQL_DATABASE:-shareo}" < "$migration"
done
