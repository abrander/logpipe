# logpipe
Small utility to enable non-syslog applications to log to the local syslog through a named pipe (FIFO).

Logpipe was developed specifically for nginx and InfluxDB, but it should be usable for any application that can log to a file, but not to syslog.
