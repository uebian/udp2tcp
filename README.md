# udp2tcp

A lightweight tool to proxy udp traffic through various TCP based protocols.

A potential use case of the tools is to setup Wireguard Link or OpenVPN Link in firewall-ed environment where only HTTP(s) traffic is cleared.

## Config

See `config.toml`

## Performance Test

Run both server (using server conf) and client (using client conf).

Run the following command to start the benchmark, increase the bandwidth `100M` to the highest possible value. 
```
socat TCP-LISTEN:40000,fork TCP:127.0.0.1:20000
iperf3 -s -p 20000
iperf3 -c 127.0.0.1 -p 40000 --cport 50000 -u --bandwidth 100M -l 1280
```