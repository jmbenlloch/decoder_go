#!/usr/bin/env bash
# Dump the DB tables the decoder needs for one run into a JSON fixture, so
# fixture-based tests can run without network access to MySQL.
#
# Usage: dump_db_fixture.sh <host> <user> <pass> <dbname> <run> <output.json>
# e.g.:  dump_db_fixture.sh next.ific.uv.es nextreader readonly DEMOPPDB 15022 \
#            pkg/testdata/demopp_15022_db.json
set -euo pipefail

HOST=$1; USER=$2; PASS=$3; DB=$4; RUN=$5; OUT=$6

q() {
    docker run --rm mysql:8.0 mysql -h "$HOST" -u "$USER" -p"$PASS" "$DB" \
        -N -B -e "$1" 2>/dev/null
}

{
    echo '{'
    echo '  "huffman_pmt": ['
    q "SELECT value, code FROM HuffmanCodesPmt WHERE MinRun <= $RUN AND MaxRun >= $RUN" |
        awk '{printf "%s    {\"value\": %s, \"code\": \"%s\"}", sep, $1, $2; sep=",\n"} END {print ""}'
    echo '  ],'
    echo '  "huffman_sipm": ['
    q "SELECT value, code FROM HuffmanCodesSipm WHERE MinRun <= $RUN AND MaxRun >= $RUN" |
        awk '{printf "%s    {\"value\": %s, \"code\": \"%s\"}", sep, $1, $2; sep=",\n"} END {print ""}'
    echo '  ],'
    echo '  "fec_elecid_base": ['
    q "SELECT FecID, BaseElecID FROM FecElecIDBase WHERE MinRun <= $RUN AND MaxRun >= $RUN" |
        awk '{printf "%s    {\"fec_id\": %s, \"base_elecid\": %s}", sep, $1, $2; sep=",\n"} END {print ""}'
    echo '  ],'
    echo '  "channel_mapping": ['
    q "SELECT cm.ElecID, cm.SensorID, cp.Label FROM ChannelMapping cm
       JOIN ChannelPosition cp ON cm.SensorID = cp.SensorID
           AND cp.MinRun <= $RUN AND cp.MaxRun >= $RUN
       WHERE cm.MinRun <= $RUN AND cm.MaxRun >= $RUN ORDER BY cm.SensorID" |
        awk '{printf "%s    {\"elec_id\": %s, \"sensor_id\": %s, \"label\": \"%s\"}", sep, $1, $2, $3; sep=",\n"} END {print ""}'
    echo '  ]'
    echo '}'
} > "$OUT"

echo "wrote $OUT"
