# Herald Runner

The Herald Runner is a http server which is used to
cooperate with the Herald Daemon executor `http_remote`.
The job from `http_remote` will be sent to the Herald Runner
and `http_remote` get result from the http response.


## Installation

Download binary file from the
[release page](https://github.com/heraldgo/herald-runner/releases).

Or install by source with [Go](https://golang.org/):

```shell
$ go get -u github.com/heraldgo/herald-runner
```


## Configuration

```yaml
log_level: INFO
log_output: /var/log/herald-runner/herald-runner.log

host: 127.0.0.1
port: 8124
#unix_socket: /var/run/herald-runner/herald-runner.sock

secret: the_secret_should_be_strong_enough

work_dir: /var/lib/herald-runner/work
```

The secret must be exactly the same as the one in `http_remote`
executor.
The `work_dir` is similar to the `local` executor.


## Run the service

Run the Herald Runner:

```shell
$ herald-runner -config config.yml
```

Press `Ctrl+C` to exit.


## HTTPS with nginx

If you would like to use https to secure the job request, you may use
reverse proxy of nginx and setup certificates there.
