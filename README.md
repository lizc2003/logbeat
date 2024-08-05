# logbeat
Based on filebeat, the output plugins of http are added, thus expanding the usage scenarios of filebeat. The output plugin of http is used to send the log to openobserve, ThinkingData(数数科技), Sensors Data(神策).

## Run
```shell
git clone --recursive https://github.com/lizc2003/logbeat.git
cd logbeat/logbeat
make
./logbeat -c config/logbeat-openobserve.yml
```
