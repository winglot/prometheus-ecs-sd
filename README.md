# prometheus-ecs-sd

A custom service discovery adapter for Prometheus that integrates with AWS ECS Services. This leverages [custom-sd](https://prometheus.io/blog/2018/07/05/implementing-custom-sd/) to output a file that can be passed as `file_sd` in prometheus.yaml. This will allow you to pass your targets deployed as ECS Services to Prometheus for scraping without having to use a static config.