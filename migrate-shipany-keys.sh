#!/usr/bin/env bash
# migrate-shipany-keys.sh
#
# One-shot migration: copy ShipAny-issued WooCommerce REST API keys out of
# the legacy WordPress site's `wp_woocommerce_api_keys` table into Gyeon's
# `legacy_wc_api_keys` table so the wcshim middleware can authenticate
# ShipAny's PUT /wp-json/wc/v3/orders/{id} callbacks.
#
# Two stages, run on different hosts:
#
#   ./migrate-shipany-keys.sh dump          # on the WC (WordPress) server
#   scp shipany-keys.tsv  gyeon-host:~/     # ship the TSV across
#   ./migrate-shipany-keys.sh load          # on the Gyeon server
#
# Schema reminder (wp_woocommerce_api_keys):
#   key_id, user_id, description, permissions, consumer_key, consumer_secret,
#   truncated_key, last_access
# where `consumer_key` is already HMAC-SHA256(plaintext_ck, "wc-api") and
# `consumer_secret` is the plaintext `cs_<32 hex>` value. See migration 087
# for the matching Postgres table.
#
# Environment overrides (all optional):
#   WC_MYSQL_DB        WC database name           (default: wordpress)
#   WC_MYSQL_USER      mysql -u value             (default: $USER)
#   WC_TABLE_PREFIX    WC table prefix            (default: wp_)
#   SHIPANY_DESC_LIKE  description LIKE filter    (default: %ShipAny%)
#   GYEON_PG_CONTAINER docker container name      (default: gyeon-postgres-1)
#   GYEON_PG_DB        gyeon db name              (default: gyeon)
#   GYEON_PG_USER      gyeon db user              (default: gyeon)
#   DUMP_FILE          intermediate TSV path      (default: shipany-keys.tsv)

set -euo pipefail

WC_MYSQL_DB="${WC_MYSQL_DB:-wordpress}"
WC_MYSQL_USER="${WC_MYSQL_USER:-${USER}}"
WC_TABLE_PREFIX="${WC_TABLE_PREFIX:-wp_}"
SHIPANY_DESC_LIKE="${SHIPANY_DESC_LIKE:-%ShipAny%}"
GYEON_PG_CONTAINER="${GYEON_PG_CONTAINER:-gyeon-postgres-1}"
GYEON_PG_DB="${GYEON_PG_DB:-gyeon}"
GYEON_PG_USER="${GYEON_PG_USER:-gyeon}"
DUMP_FILE="${DUMP_FILE:-shipany-keys.tsv}"

usage() {
    cat >&2 <<'USAGE'
Usage: migrate-shipany-keys.sh <command>

Commands:
  dump    Dump ShipAny rows from wp_woocommerce_api_keys to TSV (run on WC host)
  load    Load TSV into Gyeon's legacy_wc_api_keys table       (run on Gyeon host)
  verify  Show what's currently in Gyeon's legacy_wc_api_keys  (run on Gyeon host)

Run without arguments for this help.
USAGE
}

dump() {
    echo "→ Dumping from ${WC_TABLE_PREFIX}woocommerce_api_keys (description LIKE ${SHIPANY_DESC_LIKE}) ..."
    # --batch + --skip-column-names: clean TSV, no headers, no fancy formatting.
    # --raw: don't escape backslashes (postgres \copy reads them literally and
    # would corrupt the hashed consumer_key column).
    mysql --batch --skip-column-names --raw \
        -u "${WC_MYSQL_USER}" \
        "${WC_MYSQL_DB}" \
        -e "
            SELECT
                key_id,
                COALESCE(user_id, 0),
                COALESCE(description, ''),
                permissions,
                consumer_key,
                consumer_secret,
                COALESCE(truncated_key, ''),
                COALESCE(DATE_FORMAT(last_access, '%Y-%m-%dT%H:%i:%sZ'), '')
            FROM ${WC_TABLE_PREFIX}woocommerce_api_keys
            WHERE description LIKE '${SHIPANY_DESC_LIKE}';
        " > "${DUMP_FILE}"

    local rows
    rows=$(wc -l < "${DUMP_FILE}" | tr -d ' ')
    if [[ "${rows}" == "0" ]]; then
        echo "ERROR: no rows matched. Either ShipAny was never authorised on this WC site," >&2
        echo "       or the description filter is wrong. Try:" >&2
        echo "         SHIPANY_DESC_LIKE='%shipany%' $0 dump" >&2
        rm -f "${DUMP_FILE}"
        exit 1
    fi
    echo "✓ Wrote ${rows} row(s) to ${DUMP_FILE}"
    echo ""
    echo "Next step: scp ${DUMP_FILE} to the Gyeon host, then run:"
    echo "  ./migrate-shipany-keys.sh load"
}

load() {
    if [[ ! -s "${DUMP_FILE}" ]]; then
        echo "ERROR: ${DUMP_FILE} not found or empty. Run 'dump' on the WC host first." >&2
        exit 1
    fi

    # Use \copy from STDIN so the file doesn't have to exist inside the
    # docker container. NULL '' lets the empty last_access column become NULL
    # instead of failing the TIMESTAMPTZ parse.
    local rows
    rows=$(wc -l < "${DUMP_FILE}" | tr -d ' ')
    echo "→ Loading ${rows} row(s) into ${GYEON_PG_DB}.legacy_wc_api_keys ..."

    docker exec -i "${GYEON_PG_CONTAINER}" \
        psql -v ON_ERROR_STOP=1 \
            -U "${GYEON_PG_USER}" -d "${GYEON_PG_DB}" \
            -c "\copy legacy_wc_api_keys (key_id, user_id, description, permissions, consumer_key, consumer_secret, truncated_key, last_access) FROM STDIN WITH (FORMAT csv, DELIMITER E'\t', NULL '')" \
        < "${DUMP_FILE}"

    echo "✓ Loaded successfully."
    echo ""
    verify
}

verify() {
    echo "Current legacy_wc_api_keys rows:"
    docker exec -i "${GYEON_PG_CONTAINER}" \
        psql -U "${GYEON_PG_USER}" -d "${GYEON_PG_DB}" \
            -c "SELECT key_id, description, permissions, truncated_key, last_access, revoked_at FROM legacy_wc_api_keys ORDER BY key_id;"
}

case "${1:-}" in
    dump)   dump ;;
    load)   load ;;
    verify) verify ;;
    *)      usage; exit 1 ;;
esac
