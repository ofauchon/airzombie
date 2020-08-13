#!/bin/bash

AIRZONE_IP=192.168.1.1
INFLUX_SERVER=127.0.0.1
INFLUX_DB=metrics
INFLUX_USER=airzombie

mkdir -p logs
go run airzombie.go \
-airzone_ip=${AIRZONE_IP} \
-influx_url="http://${INFLUX_SERVER}:8086" \
-influx_db=${INFLUX_DB} \
-influx_user=${INFLUX_USER} \
-log logs/info.log 

