#!/bin/sh

# Copy existing configuration if pre-existing installation is found
if -f /etc/opt/influxdb/influxdb.conf ; then
    cp -a /etc/opt/influxdb /etc/influxdb
fi
