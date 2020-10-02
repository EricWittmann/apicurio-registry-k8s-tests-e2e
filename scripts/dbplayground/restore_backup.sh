#!/bin/bash

DB_HOST=$1
DB_PORT=$2
DB_NAME=$3
DB_USR=$4
DB_PWD=$5

echo $DB_HOST:$DB_PORT:$DB_NAME:$DB_USR:$DB_PWD >> /var/lib/pgsql/.pgpass

psql -h $DB_HOST -p $DB_PORT -U $DB_USR --no-password -d $DB_NAME < apicurio-registry-dumpfile