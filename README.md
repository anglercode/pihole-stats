# Pihole Stats ![CI](https://github.com/mcneilcode/pihole-stats/workflows/Builds/badge.svg) [![Go Report Card](https://goreportcard.com/badge/github.com/mcneilcode/pihole-stats)](https://goreportcard.com/report/github.com/mcneilcode/pihole-stats)

InfluxDB 1.7.x or older, based stats aggregation daemon for Pihole.
Collect stats from Pihole and send them into InfluxDB for automation or visualization.

## Quick Usage

**1.** Edit the .env file and update it for your environment. `PIHOLE_HOST` and the `INFLUXDB_*` vars being especially important.

**2.** `docker-compose up -d` (on first run) or `docker-compose build && docker-compose up -d` for updates.

**3.** By default the measurements are stored in InfluxDB into a store named `pihole`. This can be changed with the `INFLUXDB_MEASUREMENT` config option.

## Detailed Usage

### Direct

To compile a binary for the OS flavour of your choice:

```bash
# Linux
GOOS=linux go build -o pihole_stats .

# OSX
GOOS=darwin go build -o pihole_stats .

# Windows
GOOS=windows go build -o pihole_stats.exe .
```

Quick run:

```
INFLUXDB_HOST=192.168.2.10 PIHOLE_HOST=192.168.2.7 ./pihole_stats
```

### Docker

There is a Dockerfile provided for running pihole_stats inside a container.
After editing the .env file:


```bash
docker build -t pihole_stats:latest .
docker run -d --net=host pihole_stats:latest
```

### Docker-compose

There is also a docker-compose.yml file provided for using pihole_stats via compose.
After editing the .env file:

```bash
docker-compose build
docker-compose up -d
```


## Collected Data

The following metrics are collected from Pihole:

| Metric        | Description |
| ------------- |-------------|
|ads_percentage_today|Percentage of ads in today's DNS queries.|
|ads_blocked_today|The number of ads blocked today.|
|clients_ever_seen|The number of DNS clients that Pihole has seen or responded to.|
|domains_blocked|The number of blocked domains.|
|dns_queries_today|The total number of DNS queries today.|
|dns_queries_all_types|The total number of DNS queries, all types.|
|queries_forwarded|The amount of queries which have been forward for resolution.|
|queries_cached|The amount of queries which were serviced from DNS cache.|
|reply_CNAME|The number of CNAME replies.|
|reply_IP|The number of IP based replies.|
|reply_NODATA|The number of NODATA replies.|
|reply_NXDOMAIN|The number of NXDOMAIN replies.|
|unique_clients|The number of unique clients querying the DNS server.|
|unique_domains|The number of unique domains resolved by the DNS server.|


## Querying Data

By default the data is stored in the `pihole` store of the configured database (`telegraf` by default)
and can be queryed with the influx tool like so:

```
> use telegraf
> SELECT dns_queries_today FROM pihole LIMIT 5
name: pihole
time                dns_queries_today
----                -----------------
1608548767000000000 97037
1608548777000000000 97038
1608548787000000000 97051
1608548797000000000 97064
1608548807000000000 97077
```
