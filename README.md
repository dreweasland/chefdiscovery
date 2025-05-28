# Chef Discovery

This repository provides a Prometheus service discovery mechanism that retrieves nodes from a Chef server. It creates target groups using node information so that Prometheus can automatically scrape metrics from any Chef-managed host.

## Building and Testing

This project uses Go modules. To build or run the tests, ensure you have Go installed then execute:

```bash
go test ./...
```

## Usage

The configuration for the discoverer mirrors the fields in `SDConfig`:

```
- job_name: chef
  chef_sd_configs:
    - chef_server: https://chef.example.com
      user_id: prometheus
      user_key_file: /etc/prometheus/chef.pem
      refresh_interval: 5m
      ignore_ssl: false
      port: 9090
```

Optional `meta_attribute` entries map Chef attributes to Prometheus labels.

## License

See `LICENSE` file if present for license information.
