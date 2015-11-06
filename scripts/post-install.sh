#!/bin/sh
BIN_DIR=/usr/bin
DATA_DIR=/var/lib/influxdb

if ! id influxdb >/dev/null 2>&1; then
        useradd --system -U -M influxdb -s /bin/false -d $INFLUXDB_DATA_DIR
fi
chown influxdb:influxdb $BIN_DIR/influx*
chmod a+rX $INSTALL_ROOT_DIR/influx*

mkdir -p $INFLUXDB_LOG_DIR
chown -R -L influxdb:influxdb $INFLUXDB_LOG_DIR
mkdir -p $INFLUXDB_DATA_DIR
chown -R -L influxdb:influxdb $INFLUXDB_DATA_DIR

test -f /etc/default/influxdb || touch /etc/default/influxdb

# Remove legacy logrotate file
test -f $LOGROTATE_DIR/influxd && rm -f $LOGROTATE_DIR/influxd

# Remove legacy symlink
test -h /etc/init.d/influxdb && rm -f /etc/init.d/influxdb

# Systemd
if which systemctl > /dev/null 2>&1 ; then
    cp $INFLUXDB_SCRIPT_DIR/$SYSTEMD_SCRIPT /lib/systemd/system/influxdb.service
    systemctl enable influxdb

# Sysv
else
    cp -f $INFLUXDB_SCRIPT_DIR/$INITD_SCRIPT /etc/init.d/influxdb
    chmod +x /etc/init.d/influxdb
    if which update-rc.d > /dev/null 2>&1 ; then
        update-rc.d -f influxdb remove
        update-rc.d influxdb defaults
    else
        chkconfig --add influxdb
    fi
fi
