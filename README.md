# logbeat
Based on filebeat, the output plugins of http and clickhouse are added, thus expanding the usage scenarios of filebeat.

## Run
```shell
git clone --recursive https://github.com/lizc2003/logbeat.git
cd logbeat/logbeat
make
./logbeat -c config/logbeat-openobserve.yml
```
